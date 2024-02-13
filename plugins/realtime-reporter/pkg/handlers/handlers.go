package handlers

import "k8s.io/client-go/tools/cache"

const (
	EventAdd    string = "EventAdd"
	EventUpdate string = "EventUpdate"
	EventDelete string = "EventDelete"
)

type EventHandler func(resourceType string) cache.ResourceEventHandlerFuncs
