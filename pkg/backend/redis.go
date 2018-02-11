package backend

/*  REDIS Data Model

				Trigger-Events (Hash)

    +---------------------------------------------+
    |                                             |
    |                                             |
    | +--------------------+      +-------------+ |
    | |                    |      |             | |
    | | event:{event-uri}  +------> type        | |
    | |                    |      | kind        | |
    | |                    |      | secret      | |
    | |                    |      | endpoint    | |
    | |                    |      | description | |
    | |                    |      | help        | |
    | |                    |      | status      | |
    | |                    |      |             | |
    | +--------------------+      +-------------+ |
    |                                             |
    |                                             |
	+---------------------------------------------+

				Triggers (Sorted Set)

	+-------------------------------------------------+
	|                                                 |
	|                                                 |
	| +---------------------+     +-----------------+ |
	| |                     |     |                 | |
	| | trigger:{event-uri} +-----> {pipeline-uid}  | |
	| |                     |     |                 | |
	| +---------------------+     | ...             | |
	|                             |                 | |
	|                             | {pipeline-uid}  | |
	|                             |                 | |
	|                             +-----------------+ |
	|                                                 |
	|                                                 |
	+-------------------------------------------------+


				Pipelines (Sorted Set)

	+---------------------------------------------------+
	|                                                   |
	|                                                   |
	| +-------------------------+      +-------------+  |
	| |                         |      |             |  |
	| | pipeline:{pipeline-uid} +------> {event-uri} |  |
	| |                         |      |             |  |
	| +-------------------------+      | ...         |  |
	|                                  |             |  |
	|                                  | {event-uri} |  |
	|                                  |             |  |
	|                                  +-------------+  |
	|                                                   |
	|                                                   |
	+---------------------------------------------------+

	* event-uri     - URI unique identifier for event (specified by event provider)
    * pipeline-uid  - Codefresh pipeline UID

*/

import (
	"fmt"
	"strings"
	"time"

	"github.com/codefresh-io/hermes/pkg/codefresh"
	"github.com/codefresh-io/hermes/pkg/model"
	"github.com/codefresh-io/hermes/pkg/provider"
	"github.com/codefresh-io/hermes/pkg/util"
	"github.com/garyburd/redigo/redis"
	log "github.com/sirupsen/logrus"
)

// Redis connection pool
func newPool(server string, port int, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
			if err != nil {
				log.WithError(err).Fatal("Failed to connect to Redis store")
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					log.WithError(err).Fatal("Failed to connect to Redis store")
					return nil, err
				}
			}
			return c, nil
		},
	}
}

// RedisPool redis pool
type RedisPool struct {
	pool *redis.Pool
}

// RedisPoolService interface for getting Redis connection from pool or test mock
type RedisPoolService interface {
	GetConn() redis.Conn
}

// GetConn helper function: get Redis connection from pool; override in test
func (rp *RedisPool) GetConn() redis.Conn {
	return rp.pool.Get()
}

// RedisStore in memory trigger map store
type RedisStore struct {
	redisPool     RedisPoolService
	pipelineSvc   codefresh.PipelineService
	eventProvider provider.EventProvider
}

// helper function - discard Redis transaction and return error
func discardOnError(con redis.Conn, err error) error {
	if _, err := con.Do("DISCARD"); err != nil {
		log.WithError(err).Error("Failed to discard Redis transaction")
		return err
	}
	log.Error(err)
	return err
}

// helper function - check if string is a key and not empty or contain *
func checkSingleKey(key string) error {
	if key == "" || strings.ContainsAny(key, "?*[]^") {
		return model.ErrNotSingleKey
	}
	return nil
}

// construct prefixes - for trigger, secret and pipeline
func getPrefixKey(prefix, id string) string {
	// set * for empty id
	if id == "" {
		id = "*"
	}
	if strings.HasPrefix(id, prefix+":") {
		return id
	}
	return fmt.Sprintf("%s:%s", prefix, id)
}

func getTriggerKey(id string) string {
	return getPrefixKey("trigger", id)
}

func getPipelineKey(id string) string {
	return getPrefixKey("pipeline", id)
}

func getEventKey(id string) string {
	return getPrefixKey("event", id)
}

// NewRedisStore create new Redis DB for storing trigger map
func NewRedisStore(server string, port int, password string, pipelineSvc codefresh.PipelineService, eventProvider provider.EventProvider) *RedisStore {
	r := new(RedisStore)
	r.redisPool = &RedisPool{newPool(server, port, password)}
	r.pipelineSvc = pipelineSvc
	r.eventProvider = eventProvider
	// return RedisStore
	return r
}

//-------------------------- TriggerReaderWriter Interface -------------------------

// ListTriggersForEvents get list of defined triggers for trigger events
func (r *RedisStore) ListTriggersForEvents(events []string) ([]model.Trigger, error) {
	con := r.redisPool.GetConn()
	log.WithField("events", events).Debug("List triggers for events")

	keys := make([]string, 0)
	for _, event := range events {
		res, err := redis.Strings(con.Do("KEYS", getTriggerKey(event)))
		if err != nil {
			log.WithField("event", event).WithError(err).Error("Failed to find triggers")
			return nil, err
		}
		keys = util.MergeStrings(keys, res)
	}

	// Iterate through all trigger keys and get pipelines
	triggers := make([]model.Trigger, 0)
	for _, k := range keys {
		res, err := redis.Strings(con.Do("ZRANGE", k, 0, -1))
		if err != nil {
			log.WithField("key", k).WithError(err).Error("Failed to get pipelines")
			return nil, err
		}
		// for all linked pipelines ...
		for _, pipeline := range res {
			trigger := model.Trigger{
				Event:    strings.TrimPrefix(k, "trigger:"),
				Pipeline: pipeline,
			}
			triggers = append(triggers, trigger)
		}
	}
	return triggers, nil
}

// ListTriggersForPipelines get list of defined triggers for specified pipelines
func (r *RedisStore) ListTriggersForPipelines(pipelines []string) ([]model.Trigger, error) {
	con := r.redisPool.GetConn()
	log.WithField("pipelines", pipelines).Debug("List triggers for pipelines")

	keys := make([]string, 0)
	for _, pipeline := range pipelines {
		res, err := redis.Strings(con.Do("KEYS", getPipelineKey(pipeline)))
		if err != nil {
			log.WithField("pipeline", pipeline).WithError(err).Error("Failed to find triggers for pipeline")
			return nil, err
		}
		keys = util.MergeStrings(keys, res)
	}

	// Iterate through all pipelines keys and get trigger events
	triggers := make([]model.Trigger, 0)
	for _, k := range keys {
		res, err := redis.Strings(con.Do("ZRANGE", k, 0, -1))
		if err != nil {
			log.WithField("key", k).WithError(err).Error("Failed to get trigger events")
			return nil, err
		}
		// for all linked trigger events ...
		for _, event := range res {
			trigger := model.Trigger{
				Event:    event,
				Pipeline: strings.TrimPrefix(k, "pipeline:"),
			}
			triggers = append(triggers, trigger)
		}
	}
	return triggers, nil
}

// CreateTriggersForPipeline create trigger: link multiple events <-> pipeline
func (r *RedisStore) CreateTriggersForPipeline(pipeline string, events []string) error {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"pipeline":     pipeline,
		"event-uri(s)": events,
	}).Debug("Creating triggers")

	// check Codefresh pipeline existence
	_, err := r.pipelineSvc.CheckPipelineExists(pipeline)
	if err != nil {
		return err
	}

	// start Redis transaction
	_, err = con.Do("MULTI")
	if err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return err
	}

	// add pipeline to Triggers
	for _, event := range events {
		log.WithFields(log.Fields{
			"event-uri": event,
			"pipeline":  pipeline,
		}).Debug("Adding pipeline to the Triggers")
		_, err = con.Do("ZADD", getTriggerKey(event), 0, pipeline)
		if err != nil {
			return discardOnError(con, err)
		}
	}

	// add trigger to Pipelines
	log.WithFields(log.Fields{
		"pipeline":     pipeline,
		"event-uri(s)": events,
	}).Debug("Adding trigger to the Pipelines")
	params := []interface{}{getPipelineKey(pipeline), 0}
	params = append(params, util.InterfaceSlice(events)...)
	_, err = con.Do("ZADD", params...)
	if err != nil {
		return discardOnError(con, err)
	}

	// submit transaction
	_, err = con.Do("EXEC")
	if err != nil {
		log.WithError(err).Error("Failed to execute transaction")
	}
	return err
}

// DeleteTriggersForPipeline delete trigger: unlink multiple events from pipeline
func (r *RedisStore) DeleteTriggersForPipeline(pipeline string, events []string) error {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"pipeline":     pipeline,
		"event-uri(s)": events,
	}).Debug("Deleting triggers for pipeline")

	// start Redis transaction
	_, err := con.Do("MULTI")
	if err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return err
	}

	// remove pipeline from Triggers
	for _, event := range events {
		log.WithFields(log.Fields{
			"event-uri": event,
			"pipeline":  pipeline,
		}).Debug("Removing pipeline from the Triggers")
		_, err = con.Do("ZREM", getTriggerKey(event), pipeline)
		if err != nil {
			return discardOnError(con, err)
		}
	}

	// remove trigger(s) from Pipelines
	log.WithFields(log.Fields{
		"pipeline":     pipeline,
		"event-uri(s)": events,
	}).Debug("Removing triggers from the Pipelines")
	params := []interface{}{getPipelineKey(pipeline)}
	params = append(params, util.InterfaceSlice(events)...)
	_, err = con.Do("ZREM", params...)
	if err != nil {
		return discardOnError(con, err)
	}

	// submit transaction
	_, err = con.Do("EXEC")
	if err != nil {
		log.WithError(err).Error("Failed to execute transaction")
	}
	return err
}

// DeleteTriggersForEvent delete trigger: unlink multiple pipelines from event
func (r *RedisStore) DeleteTriggersForEvent(event string, pipelines []string) error {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"event-uri":   event,
		"pipeline(s)": pipelines,
	}).Debug("Deleting triggers for event")

	// start Redis transaction
	_, err := con.Do("MULTI")
	if err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return err
	}

	// remove event from Pipelines
	for _, pipeline := range pipelines {
		log.WithFields(log.Fields{
			"event-uri": event,
			"pipeline":  pipeline,
		}).Debug("Removing event from the Pipelines")
		_, err = con.Do("ZREM", getPipelineKey(pipeline), event)
		if err != nil {
			return discardOnError(con, err)
		}
	}

	// remove trigger(s) from Triggers
	log.WithFields(log.Fields{
		"event-uri":    event,
		"pipelines(s)": pipelines,
	}).Debug("Removing triggers from the Triggers")
	params := []interface{}{getTriggerKey(event)}
	params = append(params, util.InterfaceSlice(pipelines)...)
	_, err = con.Do("ZREM", params...)
	if err != nil {
		return discardOnError(con, err)
	}

	// submit transaction
	_, err = con.Do("EXEC")
	if err != nil {
		log.WithError(err).Error("Failed to execute transaction")
	}
	return err
}

// CreateTriggersForEvent create trigger: link event <-> multiple pipelines
func (r *RedisStore) CreateTriggersForEvent(event string, pipelines []string) error {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"event-uri": event,
		"pipelines": pipelines,
	}).Debug("Creating triggers")

	// start Redis transaction
	_, err := con.Do("MULTI")
	if err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return err
	}

	for _, pipeline := range pipelines {
		// check Codefresh pipeline existence
		_, err = r.pipelineSvc.CheckPipelineExists(pipeline)
		if err != nil {
			return discardOnError(con, err)
		}

		// add trigger to Pipelines
		log.WithFields(log.Fields{
			"pipeline":  pipeline,
			"event-uri": event,
		}).Debug("Adding trigger to Pipelines")
		_, err = con.Do("ZADD", getPipelineKey(pipeline), 0, event)
		if err != nil {
			return discardOnError(con, err)
		}
	}
	// add pipelines to Triggers
	log.WithFields(log.Fields{
		"event-uri": event,
		"pipelines": pipelines,
	}).Debug("Adding pipeline to the Triggers")
	params := []interface{}{getTriggerKey(event), 0}
	params = append(params, util.InterfaceSlice(pipelines)...)
	_, err = con.Do("ZADD", params...)
	if err != nil {
		return discardOnError(con, err)
	}

	// submit transaction
	_, err = con.Do("EXEC")
	if err != nil {
		log.WithError(err).Error("Failed to execute transaction")
	}
	return err
}

// GetPipelinesForTriggers get pipelines that have trigger defined
// can be filtered by event-uri(s)
func (r *RedisStore) GetPipelinesForTriggers(events []string) ([]string, error) {
	con := r.redisPool.GetConn()

	// accumulator array
	var all []string

	// using events filter
	if len(events) > 0 {
		for _, event := range events {
			log.WithField("event-uri", event).Debug("Getting pipelines for trigger filter")
			// get pipelines from Trigger Set
			pipelines, err := redis.Strings(con.Do("ZRANGE", getTriggerKey(event), 0, -1))
			if err != nil && err != redis.ErrNil {
				log.WithError(err).Error("Failed to get pipelines")
				return nil, err
			} else if err == redis.ErrNil {
				log.Warning("No pipelines found")
				return nil, model.ErrTriggerNotFound
			}
			if len(pipelines) == 0 {
				log.Warning("No pipelines found")
				return nil, model.ErrPipelineNotFound
			}
			// aggregate pipelines
			all = util.MergeStrings(all, pipelines)
		}
	} else { // getting all pipelines
		log.Debug("Getting all pipelines")
		// get pipelines from Trigger Set
		pipelines, err := redis.Strings(con.Do("KEYS", getPipelineKey("")))
		if err != nil && err != redis.ErrNil {
			log.WithError(err).Error("Failed to get pipelines")
			return nil, err
		} else if err == redis.ErrNil {
			log.Warning("No pipelines found")
			return nil, model.ErrTriggerNotFound
		}
		if len(pipelines) == 0 {
			log.Warning("No pipelines found")
			return nil, model.ErrPipelineNotFound
		}
		all = append(all, pipelines...)
	}

	return all, nil
}

// CreateEvent new trigger event
func (r *RedisStore) CreateEvent(eventType, kind, secret string, context string, values map[string]string) (*model.Event, error) {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"type": eventType,
		"kind": kind,
	}).Debug("Creating a new trigger event")

	// construct event URI
	eventURI, err := r.eventProvider.ConstructEventURI(eventType, kind, values)
	if err != nil {
		return nil, err
	}

	// generate random secret if required
	if secret == model.GenerateKeyword {
		log.Debug("Auto generating trigger secret")
		secret = util.RandomString(16)
	}

	// TODO: get credentials from Codefresh context
	var credentials map[string]string

	// try subscribing to event - create event in remote system through event provider
	eventInfo, err := r.eventProvider.SubscribeToEvent(eventURI, secret, credentials)
	if err != nil {
		if err == provider.ErrNotImplemented {
			// try to get event info (required method)
			eventInfo, err = r.eventProvider.GetEventInfo(eventURI, secret)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	event := model.Event{
		URI:       eventURI,
		Type:      eventType,
		Kind:      kind,
		Secret:    secret,
		EventInfo: *eventInfo,
	}

	// start Redis transaction
	if _, err := con.Do("MULTI"); err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return nil, err
	}
	// store event type
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "type", eventType); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event kind
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "kind", kind); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event secret
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "secret", secret); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event description (from event provider)
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "description", eventInfo.Description); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event endpoint (from event provider)
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "endpoint", eventInfo.Endpoint); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event help (from event provider)
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "help", eventInfo.Help); err != nil {
		return nil, discardOnError(con, err)
	}
	// store event status (from event provider)
	if _, err := con.Do("HSETNX", getEventKey(eventURI), "status", eventInfo.Status); err != nil {
		return nil, discardOnError(con, err)
	}
	// submit transaction
	if _, err := con.Do("EXEC"); err != nil {
		log.WithError(err).Error("Failed to execute transaction")
		return nil, err
	}
	return &event, nil
}

// GetSecret trigger by eventURI
func (r *RedisStore) GetSecret(eventURI string) (string, error) {
	con := r.redisPool.GetConn()
	log.WithField("event-uri", eventURI).Debug("Getting trigger secret")
	// get secret from String
	secret, err := redis.String(con.Do("HGET", getEventKey(eventURI), "secret"))
	if err != nil && err != redis.ErrNil {
		log.WithError(err).Error("Failed to get secret")
		return "", err
	}
	return secret, nil
}

// GetEvent get event by event URI
func (r *RedisStore) GetEvent(event string) (*model.Event, error) {
	con := r.redisPool.GetConn()
	log.WithField("event-uri", event).Debug("Getting trigger event")
	// check event URI
	if err := checkSingleKey(event); err != nil {
		return nil, err
	}
	// get hash values
	fields, err := redis.StringMap(con.Do("HGETALL", getEventKey(event)))
	if err != nil {
		log.WithError(err).Error("Failed to get trigger event fields")
		return nil, err
	}
	if len(fields) == 0 {
		log.Error("Failed to find trigger event")
		return nil, model.ErrEventNotFound
	}
	return model.StringsMapToEvent(event, fields), nil
}

// GetEvents get events by event type, kind and filter (can be URI or part of URI)
func (r *RedisStore) GetEvents(eventType, kind, filter string) ([]model.Event, error) {
	con := r.redisPool.GetConn()
	log.WithFields(log.Fields{
		"type":   eventType,
		"kind":   kind,
		"filter": filter,
	}).Debug("Getting trigger events")
	// get all events URIs
	uris, err := redis.Strings(con.Do("KEYS", getEventKey(filter)))
	if err != nil {
		log.WithError(err).Error("Failed to get trigger events")
		return nil, err
	}
	// scan through all events and select matching to non-empty type and kind
	events := make([]model.Event, 0)
	for _, uri := range uris {
		event, err := r.GetEvent(uri)
		if err != nil {
			return nil, err
		}
		if (eventType == "" || event.Type == eventType) &&
			(kind == "" || event.Kind == kind) {
			events = append(events, *event)
		}
	}
	return events, nil
}

// DeleteEvent delete trigger event
func (r *RedisStore) DeleteEvent(event string, context string) error {
	con := r.redisPool.GetConn()
	log.WithField("event-uri", event).Debug("Deleting trigger event")
	// check event URI
	if err := checkSingleKey(event); err != nil {
		return err
	}
	// TODO: get credentials from Codefresh context
	// credentials := make(map[string]string)

	// get pipelines linked to the trigger event
	pipelines, err := redis.Strings(con.Do("ZRANGE", getTriggerKey(event), 0, -1))
	if err != nil {
		log.WithError(err).Error("Failed to get pipelines for the trigger event")
		return err
	}

	// start Redis transaction
	_, err = con.Do("MULTI")
	if err != nil {
		log.WithError(err).Error("Failed to start Redis transaction")
		return err
	}

	// delete event hash for key
	log.Debug("Removing trigger event")
	_, err = con.Do("DEL", getEventKey(event))
	if err != nil {
		log.WithError(err).Error("Failed to delete trigger event")
		return discardOnError(con, err)
	}

	// delete trigger event from Triggers
	log.Debug("Removing trigger event from Triggers")
	_, err = con.Do("DEL", getTriggerKey(event))
	if err != nil {
		return discardOnError(con, err)
	}

	// remove trigger event from linked Pipelines
	for _, pipeline := range pipelines {
		log.WithFields(log.Fields{
			"pipeline":  pipeline,
			"event-uri": event,
		}).Debug("Removing trigger event from Pipelines")
		_, err = con.Do("ZREM", getPipelineKey(pipeline), event)
		if err != nil {
			return discardOnError(con, err)
		}
	}

	// submit transaction
	_, err = con.Do("EXEC")
	if err != nil {
		log.WithError(err).Error("Failed to execute transaction")
	}

	return err
}

//-------------------------- Pinger Interface -------------------------

// Ping Redis services
func (r *RedisStore) Ping() (string, error) {
	con := r.redisPool.GetConn()
	// get pong from Redis
	pong, err := redis.String(con.Do("PING"))
	if err != nil {
		log.WithError(err).Error("Failed to ping Redis server")
	}
	return pong, err
}
