package mydeployment

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	k8slisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod"
	"knative.dev/pkg/configmap"
	knativeController "knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	knativeReconciler "knative.dev/pkg/reconciler"
	"knative.dev/sample-controller/pkg/apis/demo"
	v1 "knative.dev/sample-controller/pkg/apis/demo/v1"
	mydeploymentinformers "knative.dev/sample-controller/pkg/client/injection/informers/demo/v1/mydeployment"
	mydeploymentReconciler "knative.dev/sample-controller/pkg/client/injection/reconciler/demo/v1/mydeployment"
	"strconv"
)

type Reconciler struct {
	// 能够列出当前pod的接口
	PodLister k8slisters.PodLister
	// k8s API, 能够增删改查api
	kubeClient kubernetes.Interface
}

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *knativeController.Impl {

	podInformer := podinformer.Get(ctx)
	mydeploymentInformers := mydeploymentinformers.Get(ctx)

	reconciler := &Reconciler{
		podInformer.Lister(),
		kubeclient.Get(ctx),
	}

	impl := mydeploymentReconciler.NewImpl(ctx, reconciler)
	mydeploymentInformers.Informer().AddEventHandler(knativeController.HandleAll(impl.Enqueue))
	// Listen for events on the child resources and enqueue the owner of them.
	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: knativeController.FilterController(&v1.MyDeployment{}),
		Handler:    knativeController.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

func (r Reconciler) ReconcileKind(ctx context.Context, o *v1.MyDeployment) knativeReconciler.Event {

	logger := logging.FromContext(ctx)

	ns := o.Namespace
	podList, err := r.PodLister.Pods(ns).List(labels.SelectorFromSet(labels.Set{}))
	if err != nil {
		return fmt.Errorf("failed to list existing pods: %w", err)
	}
	logger.Infof("Found %d pods in total", len(podList))
	replicas, err := strconv.Atoi(o.Spec.Replicas)
	toCreate := replicas - len(podList)
	logger.Infof("Got %d existing pods, desired  Replicas is %d", len(podList), replicas)
	if toCreate > 0 {
		// 需要创建pod
		logger.Infof("need create %d pods", toCreate)
		pods := makePods(o)
		for i := 0; i < toCreate; i++ {
			_, err := r.kubeClient.CoreV1().Pods(pods.Namespace).Create(ctx, pods, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create pod: %w", err)
			}
		}
	} else if toCreate < 0 {
		//多余的pod， 需要删除
		toDelete := toCreate * -1
		logger.Infof("need delete %d pods", toDelete)
		for i := 0; i < toDelete; i++ {
			r.kubeClient.CoreV1().Pods(o.Namespace).Delete(ctx, podList[i].Name, metav1.DeleteOptions{})
		}
	}

	return nil
}

func makePods(d *v1.MyDeployment) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    d.ObjectMeta.Namespace,
			GenerateName: d.Name + "-",
			Labels: map[string]string{
				// The label allows for easy querying of all the pods created.
				demo.GroupName + "/podOwner": d.Name,
			},
			// The OwnerReference makes sure the pods get removed automatically once the
			// SimpleDeployment is removed.
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(d)},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "abreaking-container", Image: d.Spec.Image},
			},
		},
	}
}

var (
	_ mydeploymentReconciler.Interface = (*Reconciler)(nil)
)
