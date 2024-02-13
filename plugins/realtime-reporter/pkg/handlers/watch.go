package handlers

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

func WatchHandler(resourceType string) cache.ResourceEventHandlerFuncs {

	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		logrus.WithField("resourceType", resourceType).WithField("obj", obj).Info("add event")
	}
	handler.UpdateFunc = func(old, new interface{}) {
		logrus.WithField("resourceType", resourceType).WithField("old", old).WithField("new", new).Info("update event")
	}
	handler.DeleteFunc = func(obj interface{}) {
		logrus.WithField("resourceType", resourceType).WithField("obj", obj).Info("delete event")
	}
	return handler
}
