package main

import (
	"flag"
	"github.com/golang/glog"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientset "nfs-controller/pkg/client/clientset/versioned"
	nvinformer "nfs-controller/pkg/client/informers/externalversions"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var (
	masterURL  string
	kubeConfig string
)

func init() {
	flag.StringVar(&kubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	flag.Parse()
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		glog.Fatalf("Failed to build kubeconfig: %s", err.Error())
	}

	kubeClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Failed to build kubernetes clientset: %s", err.Error())
	}

	nvClientSet, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Failed to build nfs volume clientset: %s", err.Error())
	}

	informersFactory := informers.NewSharedInformerFactory(kubeClientSet, time.Second*30)
	nvInformerFactory := nvinformer.NewSharedInformerFactory(nvClientSet, time.Second*30)
	ctrl := NewController(kubeClientSet,
		nvClientSet,
		nvInformerFactory.Samplecrd().V1().NFSVolumes(),
		informersFactory.Core().V1().PersistentVolumes(),
		informersFactory.Core().V1().PersistentVolumeClaims(),
	)

	stopCh := sigHandler()

	go informersFactory.Start(stopCh)
	go nvInformerFactory.Start(stopCh)
	err = ctrl.Run(runtime.GOMAXPROCS(0), stopCh)
	if err != nil {
		glog.Fatalf("Failed to run controller: %s", err.Error())
	}
	<-stopCh
}

func sigHandler() <-chan struct{} {
	stopCh := make(chan struct{})
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(stopCh)
		<-sigCh // second signal. Exit directly
		os.Exit(-1)
	}()
	return stopCh
}
