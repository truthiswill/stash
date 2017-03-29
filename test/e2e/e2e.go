package test

import (
	"errors"
	"os/user"
	"path/filepath"
	"time"

	"github.com/appscode/log"
	"github.com/appscode/restik/pkg/controller"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

var image = "appscode/restik:latest"

func runController() (*controller.Controller, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube/config"))
	if err != nil {
		return &controller.Controller{}, err
	}
	controller := controller.New(config, image)
	go controller.RunAndHold()
	return controller, nil
}

func checkEventForBackup(watcher *controller.Controller, eventName string) error {
	var err error
	event := &api.Event{}
	try := 0
	for {
		event, err = watcher.Client.Core().Events(namespace).Get(eventName)
		if err == nil {
			break
		}
		if try > 12 {
			return err
		}
		log.Infoln("Waiting for 10 second for events of backup process")
		time.Sleep(time.Second * 10)
		try++
	}
	if event.Reason == "Failed" {
		return errors.New("Restic backup failed.")
	}
	return err
}

func checkContainerAfterBackupDelete(watcher *controller.Controller, name string, _type string) error {
	try := 0
	var err error
	var containers []api.Container
	for {
		log.Infoln("Waiting 20 sec for checking restik-sedecar deletion")
		time.Sleep(time.Second * 20)
		switch _type {
		case controller.ReplicationController:
			rc, err := watcher.Client.Core().ReplicationControllers(namespace).Get(name)
			if err != nil {
				containers = rc.Spec.Template.Spec.Containers
			}
		case controller.ReplicaSet:
			rs, err := watcher.Client.Extensions().ReplicaSets(namespace).Get(name)
			if err != nil {
				containers = rs.Spec.Template.Spec.Containers
			}
		case controller.Deployment:
			deployment, err := watcher.Client.Extensions().Deployments(namespace).Get(name)
			if err != nil {
				containers = deployment.Spec.Template.Spec.Containers
			}

		case controller.DaemonSet:
			daemonset, err := watcher.Client.Extensions().DaemonSets(namespace).Get(name)
			if err != nil {
				containers = daemonset.Spec.Template.Spec.Containers
			}
		}
		err = checkContainerDeletion(containers)
		if err == nil {
			break
		}
		try++
		if try > 6 {
			break
		}
	}
	return err
}

func checkContainerDeletion(containers []api.Container) error {
	for _, c := range containers {
		if c.Name == controller.ContainerName {
			return errors.New("ERROR: Restik sidecar not deleted")
		}
	}
	return nil
}
