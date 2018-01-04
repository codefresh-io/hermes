package controller

import (
	"net/http"

	"github.com/codefresh-io/hermes/pkg/backend"
	"github.com/codefresh-io/hermes/pkg/model"
	"github.com/gin-gonic/gin"
)

// EventController trigger controller
type EventController struct {
	trigger              model.TriggerReaderWriter
	eventHandlerInformer backend.EventHandlerInformer
}

// NewEventController new trigger controller
func NewEventController(trigger model.TriggerReaderWriter, eventHandlerInformer backend.EventHandlerInformer) *EventController {
	return &EventController{trigger, eventHandlerInformer}
}

// ListTypes get registered trigger types
func (c *EventController) ListTypes(ctx *gin.Context) {
	types := c.eventHandlerInformer.GetTypes()
	if types == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "no types found"})
		return
	}

	ctx.JSON(http.StatusOK, types)
}

// GetType get details for specific trigger type
func (c *EventController) GetType(ctx *gin.Context) {
	// get event type and kind
	eventType := ctx.Param("type")
	if eventType == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "missing event type"})
		return
	}
	// get event kind
	eventKind := ctx.Param("kind")

	typeObject := c.eventHandlerInformer.GetType(eventType, eventKind)
	if typeObject == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "failed to find type " + eventType + " of kind " + eventKind})
		return
	}

	ctx.JSON(http.StatusOK, typeObject)

}

// GetEventInfo get human readable text for event (ask Event Provider)
func (c *EventController) GetEventInfo(ctx *gin.Context) {
	// get event URI
	eventURI := ctx.Param("id")
	if eventURI == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "missing event uri"})
		return
	}

	// get secret by eventURI
	secret, err := c.trigger.GetSecret(eventURI)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "failed to get secret for event", "error": err.Error()})
		return
	}

	info := c.eventHandlerInformer.GetEventInfo(eventURI, secret)
	if info == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "message": "failed to find info for event " + eventURI})
		return
	}

	ctx.JSON(http.StatusOK, info)
}
