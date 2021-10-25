package main

import (
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func createClient() *kubernetes.Clientset {
	// setup client, first by...
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) < 1 {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config") // getting ~/.kube/config
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig) // use kubeconfig if possible
	if err != nil {                                               // otherwise,
		config, err = rest.InClusterConfig() // fall back to in-cluster config
		if err != nil {                      // if we still can't get a kubernetes api connection
			log.Fatalln(err) // fail completely
		} // we have an in-cluster config
	} // we have some kind of working config

	clientset, err := kubernetes.NewForConfig(config) // this is our client for the kubernetes api
	if err != nil {                                   // if we can't get a client to work
		log.Fatalln(err) // fail completely
	} // we have a working client

	return clientset
}
