package util

import (
	"fmt"
	"strings"
)

// MergeStrings merge string slices
func MergeStrings(a, b []string) []string {
	for _, bv := range b {
		found := false
		for _, av := range a {
			if av == bv {
				found = true
				break
			}
		}
		if !found {
			a = append(a, bv)
		}
	}
	return a
}

// DiffStrings returns the elements in a that aren't in b
func DiffStrings(a, b []string) []string {
	mb := map[string]bool{}
	for _, x := range b {
		mb[x] = true
	}
	ab := []string{}
	for _, x := range a {
		if _, ok := mb[x]; !ok {
			ab = append(ab, x)
		}
	}
	return ab
}

// InterfaceSlice helper function to convert []string to []interface{}
// see https://github.com/golang/go/wiki/InterfaceSlice
func InterfaceSlice(slice []string) []interface{} {
	islice := make([]interface{}, len(slice))
	for i, v := range slice {
		islice[i] = v
	}
	return islice
}

// StringSliceToMap convert string slice (with key=value strings) to map
func StringSliceToMap(values []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, v := range values {
		kv := strings.Split(v, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("unexpected 'value: %s ; should be in 'key=value' form", v)
		}
		result[kv[0]] = kv[1]
	}
	return result, nil
}
