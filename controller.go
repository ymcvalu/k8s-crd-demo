package main

import (
	"fmt"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	nvapis "nfs-controller/pkg/apis/samplecrd/v1"
	clientset "nfs-controller/pkg/client/clientset/versioned"
	nvScheme "nfs-controller/pkg/client/clientset/versioned/scheme"
	informers "nfs-controller/pkg/client/informers/externalversions/samplecrd/v1"
	listers "nfs-controller/pkg/client/listers/samplecrd/v1"
)

const controllerAgentName = "nfs-volume-controller"

type Controller struct {
	kubeClientset   kubernetes.Interface // 访问k8s原生资源
	nfsVolClientset clientset.Interface  // 访问自定义nfsVolume资源

	nfsVolumeLister listers.NFSVolumeLister
	nfsVolumeSynced cache.InformerSynced

	pvLister listers1.PersistentVolumeLister
	pvSynced cache.InformerSynced

	pvcLister listers1.PersistentVolumeClaimLister
	pvcSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewController(kubeClientSet kubernetes.Interface,
	nfsVolumeClientset clientset.Interface,
	nfsVolumeInformer informers.NFSVolumeInformer,
	pvInformer informers1.PersistentVolumeInformer,
	pvcInformer informers1.PersistentVolumeClaimInformer,
) *Controller {

	utilruntime.Must(nvScheme.AddToScheme(scheme.Scheme))

	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	ctrl := &Controller{
		kubeClientset:   kubeClientSet,
		nfsVolClientset: nfsVolumeClientset,
		nfsVolumeLister: nfsVolumeInformer.Lister(),
		nfsVolumeSynced: nfsVolumeInformer.Informer().HasSynced,
		pvLister:        pvInformer.Lister(),
		pvSynced:        pvInformer.Informer().HasSynced,
		pvcLister:       pvcInformer.Lister(),
		pvcSynced:       pvcInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NFSVolume"),
		recorder:        recorder,
	}

	// add handler
	nfsVolumeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ctrl.enqueueNFSVolume(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			old := oldObj.(*nvapis.NFSVolume)
			new := newObj.(*nvapis.NFSVolume)
			if old.ResourceVersion == new.ResourceVersion {
				// Periodic resync will send update events for all known Networks.
				// Two different versions of the same Network will always have different RVs.
				return
			}
			ctrl.enqueueNFSVolume(new)
		},
		DeleteFunc: func(obj interface{}) {
			ctrl.enqueueNFSVolumeForDelete(obj)
		},
	})

	return ctrl
}

func (ctrl *Controller) Run(nworker int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer ctrl.workqueue.ShutDown()

	glog.Info("Starting the nfs volume control loop")
	glog.Info("Waiting for  caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, ctrl.nfsVolumeSynced); !ok {
		return fmt.Errorf("failed to wait for nfs volume caches to sync")
	}
	if ok := cache.WaitForCacheSync(stopCh, ctrl.pvSynced); !ok {
		return fmt.Errorf("failed to wait for pv caches to sync")
	}
	if ok := cache.WaitForCacheSync(stopCh, ctrl.pvcSynced); !ok {
		return fmt.Errorf("failed to wait for pvc caches to sync")
	}

	glog.Info("Staring workers")
	for i := 0; i < nworker; i++ {
		go ctrl.runWorker()
	}

	glog.Info("Started workers")

	return nil
}

func (ctrl *Controller) runWorker() {
	for ctrl.processNext() {
	}
}

func (ctrl *Controller) processNext() bool {
	it, shutdown := ctrl.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer ctrl.workqueue.Done(obj)

		key, ok := obj.(string)
		if !ok {
			ctrl.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		err := ctrl.handler(key)
		if err != nil {
			return fmt.Errorf("failed to sync '%s': %s", key, err.Error())
		}

		ctrl.workqueue.Forget(obj)
		glog.Info("Successfully synced '%s'", key)

		return nil
	}(it)

	if err != nil {
		utilruntime.HandleError(err)
	}

	return true
}

func (ctrl *Controller) handler(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}

	nv, err := ctrl.nfsVolumeLister.NFSVolumes(ns).Get(name)
	if err != nil && errors.IsNotFound(err) {
		ctrl.delNFSVolume(ns, name)
	} else if err != nil {
		return err
	}

	ctrl.upsert(nv)

	return nil
}

func (ctrl *Controller) upsert(nv *nvapis.NFSVolume) error {
	// TODO: diff and update/create pv&pvc
	return nil
}

func (ctrl *Controller) delNFSVolume(ns, name string) error {
	pvAppLabel := name + "_pv"
	pvcAppLabel := name + "_pvc"

	// TODO: add backoff retry and handle error
	pvcs, err := ctrl.pvcLister.PersistentVolumeClaims(ns).List(labels.SelectorFromSet(labels.Set{"app": pvcAppLabel}))
	if err != nil {
		return err
	}
	if len(pvcs) > 0 {
		err := ctrl.kubeClientset.CoreV1().PersistentVolumeClaims(ns).Delete(pvcs[0].Name, nil)
		if err != nil {
			return err
		}
	}

	pvs, err := ctrl.pvLister.List(labels.SelectorFromSet(labels.Set{"app": pvAppLabel}))
	if err != nil {
		return err
	}
	if len(pvs) > 0 {
		err := ctrl.kubeClientset.CoreV1().PersistentVolumes().Delete(pvs[0].Name, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *Controller) enqueueNFSVolume(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	ctrl.workqueue.AddRateLimited(key)
}

func (ctrl *Controller) enqueueNFSVolumeForDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	ctrl.workqueue.AddRateLimited(key)
}
