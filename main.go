package main

import (
	"flag"
	"log"
	"path/filepath"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

var (
	startTime = metav1.Now()
	watchPods = make(map[types.UID]interface{}, 0)
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Pods().Informer()
	stopper := make(chan struct{})
	defer close(stopper)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			mObj := obj.(*v1.Pod)
			addPod(mObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			mNewObj := newObj.(*v1.Pod)
			// log.Printf("Update: %s, %s", mOldObj, mNewObj)
			updatePod(mNewObj)
		},
		DeleteFunc: func(obj interface{}) {
			// "k8s.io/apimachinery/pkg/apis/meta/v1" provides an Object
			// interface that allows us to get metadata easily
			mObj := obj.(*v1.Pod)
			deletePod(mObj)
		},
	})

	informer.Run(stopper)
}

func addPod(pod *v1.Pod) {
	if startTime.Time.After(pod.CreationTimestamp.Time) {
		return
	}

	if podutil.IsPodReady(pod) {
		ready := podutil.GetPodReadyCondition(pod.Status)
		duration := ready.LastTransitionTime.Time.Sub(pod.CreationTimestamp.Time)
		log.Printf("startup duration of %s: %s", pod.GetName(), duration.String())
	} else {
		watchPods[pod.UID] = struct{}{}
	}
}

func updatePod(pod *v1.Pod) {
	if podutil.IsPodReady(pod) {
		if _, ok := watchPods[pod.UID]; ok {
			ready := podutil.GetPodReadyCondition(pod.Status)
			duration := ready.LastTransitionTime.Time.Sub(pod.CreationTimestamp.Time)
			log.Printf("startup duration of %s: %s", pod.GetName(), duration.String())
			delete(watchPods, pod.UID)
		}
	}
}

func deletePod(pod *v1.Pod) {
	if _, ok := watchPods[pod.UID]; ok {
		delete(watchPods, pod.UID)
	}
}
