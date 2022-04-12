基于Knative自定义controller开发



# Controller概述

## operator=crd+controller

kubernetes，我们可以自定义资源（CustomResourceDefinition，简称CRD）。比如kubernetes上常见的pod、service、deployment等都是CRD，只不过这些都是kubernetes系统级自带的CRD资源。

为了可以封装、管理和部署我们自定义的Kubernetes 应用，kubernetes提供了operator方式。Operator 是使用自定义资源（CR）管理应用及其组件的自定义 Kubernetes 控制器（Controller）。

> 注：operator貌似并不是kubernetes推出来的，只不过后来kubernetes接受了operator，具体过程可以自行百度。

简单的说：operator=crd+controller

从operator引申出来的方式比较多，比如go开发经常使用的[Kubebuilder](https://book.kubebuilder.io/cronjob-tutorial/basic-project.html)、java开发可使用的[fabric8io](https://github.com/fabric8io/kubernetes-client)。这两种方式都值得研究，不过由于kubernetes本身就是go语言开发的，可能用Kubebuilder会比较多一点。

此外呢，使用Knative来开发controller貌似更简单一些。



创建一个完整的CRD，一半需要有两部分：

1. crd资源内容如何定义？——CustomResource
2. 资源有增删改查的操作如何进行控制？—— Controller

所以，第2点就是Controller需要做的事情。



knative提供了更简单的controller开发框架，那么本文将介绍如何通过knative来自定义CRD和开发Controller。



## 为什么要自定义Controller开发

控制器的工作是确保对于任何给定的对象，世界的实际状态（包括集群状态，以及潜在的外部状态，如 Kubelet 的运行容器或云提供商的负载均衡器）与对象中的期望状态相匹配。每个控制器专注于一个根 Kind，但可能会与其他 Kind 交互。

我们把这个过程称为 **reconciling**。



在 controller-runtime 中，为特定种类实现 reconciling 的逻辑被称为 [*Reconciler*](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile)。 Reconciler 接受一个对象的名称，并返回我们是否需要再次尝试（例如在错误或周期性控制器的情况下，如 HorizontalPodAutoscaler）。

# Controller模板

knative官方提供了一个Controller开发的模板：[knative-sandbox/sample-controller](https://github.com/knative-sandbox/sample-controller)，只需要`user this template`即可：

![image-20220331161130646](C:\Users\MI\AppData\Roaming\Typora\typora-user-images\image-20220331161130646.png)



然后填写你自己的仓库名，就可以生成了controller的样例[代码 knative-sample-controller](https://github.com/aBreaking/knative-sample-controller)了。

生成代码的主要目录结构如下：

```yaml
knative-sample-controller
├── cmd # 包含 controller 和webhook 的入口 main 函数,以及生成 crd  的 schema 工具
│   ├── controller 
│   │   └── main.go # controller 的启动入口文件
│   ├── schema
│   │   └── main.go # 生成 CRD 资源的 工具
│   └── webhook
│       └── main.go # webhook 的入口文件
├── config # controller 和webhook 的部署文件（deploy role clusterrole 等等，此处省略）
│   ├── 300-addressableservice.yaml
│   ├── 300-simpledeployment.yaml
├── example-addressable-service.yaml # CR 资源的示例yaml
├── example-simple-deployment.yaml # CR 资源的示例yaml
├── hack # 是 程序自动生成代码的脚本，其中的 update-codegen.sh 最常用
│   ├── update-codegen.sh # 生成 informer，clientset，injection，lister的工具
│   ├── update-deps.sh
│   ├── update-k8s-deps.sh
│   └── verify-codegen.sh
├── pkg 
│   ├── apis # CRD 定义的 types 文件
│   │   └── samples 
│   │       ├── register.go
│   │       └── v1alpha1 # 此处需编写 CRD 资源的types
│   ├── client # 执行 hack/update-codegen.sh 后自动生成的文件
│   │   ├── clientset
│   │   ├── informers
│   │   ├── injection
│   │   └── listers
│   └── reconciler # 此处是控制器的主要逻辑，示例中实现了两个控制器，每个控制器包含主控制器入口（controller.go） 和对应的 reconcile 逻辑
│       ├── addressableservice
│       │   ├── addressableservice.go
│       │   └── controller.go
│       └── simpledeployment
│           ├── controller.go
│           └── simpledeployment.go
```



从代码上看，knative-sample-controller提供了两个默认的controller实现：addressableservice和simpledeployment。这里呢，我们来仿照simpledeployment来定义crd和实现controller 的编码。



# CRD定义

## 确定GKV

在kubernetes中谈论API时，通常会说到这几个术语：groups（组 ）、versions（版本）、kinds（类型），还有一个resources（资源）。

group简单来说就是相关功能的集合。每个组都有一个或多个version，顾名思义，它允许我们随着时间的推移改变 API 的职责。

每个 API 组-版本包含一个或多个 API 类型，称之为 Kinds。

偶尔也会听到 resources。 resources 只是 API 中的一个 Kind 的使用方式。通常情况下，Kind 和 resources 之间有一个一对一的映射。

**GVK** = Group Version Kind

**GVR** = Group Version Resources



那么这里我定义：

* `group`为`demo.abreaking.com`

* `kind`为 `MyDeployment`

* `version`为`v1`

当在一个特定的群组版本 (Group-Version) 中提到一个 Kind 时，我们会把它称为 **GroupVersionKind**，简称 GVK。每个 GVK 对应 Golang 代码中的到对应生成代码中的 Go type。



## 创建API

1. 首先我们在`pkg/apis/demo/register.go`文件中确定要注册的`GroupName`：

   ```go
   // pkg/apis/demo/register.go
   package samples
   
   const (
   	// GroupName is the name of the API group.
   	GroupName = "demo.abreaking.com"
   )
   ```



2. CRD资源的配置

   从原来的模板代码中我们可以看到，对于每个要定义的CRD资源，都有这四个文件：

   * xxx_types.go：定义我们的CRD对象属性
   * xxx_validation.go:  用于 `webhook` 校验
   * xxx_lifecycle.go: 用于`status` 状态的设置
   * xxx_defaults.go: 用于 默认值的设置

   为此，我们直接仿照源码的模板代码创建这4个文件”

   ```shell
   pkg/apis
   ├── demo # GroupName的第一个单词
   │   ├── v1  # 版本
   |	│   ├── my_deployment_types.go
   |	│   ├── my_deployment_lifecycle.go
   |	│   ├── my_deployment_validation.go
   │   │   └── my_deployment_defaults.go
   ```

   **简单起见，后面只编写 types文件。**

3. 编写CRD types文件

   在`pkg/apis/demo/v1`目录下，创建自定义的types文件：my_deployment_types.go。

   这里实现一个简单的deployment，内容如下：

   ```go
   // MyDeployment 下面的gen注释作用是后面用来生成代码
   //
   // +genclient
   // +genreconciler
   // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
   type MyDeployment struct {
   
   	metav1.TypeMeta `json:",inline"`
   	
   	metav1.ObjectMeta `json:"metadata,omitempty"`
   
   	Spec MyDeploymentSpec `json:"spec,omitempty"`
   
   	Status MyDeploymentStatus `json:"status,omitempty"`
   }
   
   // MyDeploymentSpec 指定Myployment期望达到什么样的状态
   type MyDeploymentSpec struct {
   	Image string `json:"image"`
   	Replicas string `json:"replicas"`
   }
   
   type MyDeploymentStatus struct {
   	duckv1.Status `json:",inline"`
   	ReadyReplicas int32 `json:"readyReplicas"`
   }
   ```

   然后呢，我们还需给`MyDeployment`再实现几个接口，通过如下代码可以结合idea自动补全代码：

   ```
   var (
   	_ apis.Validatable = (*MyDeployment)(nil)
   	_ apis.Defaultable = (*MyDeployment)(nil)
   	_ kmeta.OwnerRefable = (*MyDeployment)(nil)
   	_ duckv1.KRShaped = (*MyDeployment)(nil)
   )
   ```

   然后通过类似的方式自动补全要实现接口的方法（可能各个idea方式不太一样）：

   ![image-20220412102759923](C:\Users\MI\AppData\Roaming\Typora\typora-user-images\image-20220412102759923.png)

   补全的方法代码如下：

   ```go
   // my_deployment_types.go
   
   
   func (m MyDeployment) GetStatus() *duckv1.Status {
   	return &m.Status.Status
   }
   
   func (m MyDeployment) GetConditionSet() apis.ConditionSet {
   	return apis.ConditionSet{}
   }
   
   func (m MyDeployment) GetGroupVersionKind() schema.GroupVersionKind {
   	return SchemeGroupVersion.WithKind("MyDeployment")
   }
   
   func (m MyDeployment) SetDefaults(ctx context.Context) {
   
   }
   // Validate 可以做一个简单的校验，比如判断传入image内容
   func (m MyDeployment) Validate(ctx context.Context) *apis.FieldError {
   	fmt.Println("image is ",m.Spec.Image)
   	if m.Spec.Image == ""{
   		return apis.ErrMissingField("image")
   	}
   	return nil
   }
   ```



4.

## update-codegen.sh

CRD资源定义完毕后，



首先，需要在`pkg/apis/demo/v1`目录下创建一个`doc.go`文件，里面的内容只需要指定：

```go
// pkg/apis/demo/v1/doc.go
package v1
```



打开`hack/update-codegen.sh`文件，修改我们要生成crd文件的路径：

```shell
# hack/update-codegen.sh
###  前面内容略 ###

## demo:v1 就是我们前面定义的type文件所在位置，下同
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  knative.dev/sample-controller/pkg/client knative.dev/sample-controller/pkg/apis \
  "demo:v1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt


${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  knative.dev/sample-controller/pkg/client knative.dev/sample-controller/pkg/apis \
  "demo:v1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

###  后面内容略 ###

```



然后手动执行`./hack/update-codegen.sh`命令，即可生成了`zz_generated.deepcopy.go`文件和`clinet`目录，位置如下：

```shell
knative-sample-controller
├── pkg 
│   ├── apis 
│   │   └── demo 
│   │       └── v1
|   |            └── zz_generated.deepcopy.go # 生成的深拷贝文件
│   ├── client  # 生成的client目录和下面的子目录文件
│   │   ├── clientset
│   │   ├── informers
│   │   ├── injection
│   │   └── listers
```



> 注：执行`update-codegen.sh`脚本可能会遇到跨平台的问题，如果你是在windows上开发，然后在linux（比如ubuntu on windows）上执行该脚本可能会出现换行符之类的报错。
>
> 这是因为不同操作系统之前换行符的问题。Windows格式文件的换行符为\r\n ,而Unix&Linux文件的换行符为\n。
>
> 解决该问题需要使用到`dos2unix`命令，该命令安装方式也比较简单，直接`apt install dos2unix`即可。 然后执行`dos2unix *.sh`命令即可。
>



# 控制器逻辑编写

## controller入口

程序启动总的需要从main函数中进入，`cmd/controller/main.go`，该文件就是controller的入口。内容：

```go
package main

import (
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/sample-controller/pkg/reconciler/mydeployment"
)

func main() {
	sharedmain.Main(
		"controller",
		mydeployment.NewController,
	)
}
```

`mydeployment.NewController`就是等下我们需要自定义的控制器逻辑。

先说下`sharedmain.Main`的作用，启动后，该方法会做以下事情：

1. 启动各种 `informer`，启动 所有 `controller`， `knative.dev/pkg/injection/sharedmain/main.go#238`

2. 执行工作流 `processNextWorkItem` ，`knative.dev/pkg/injection/sharedmain/main.go#468`

3. 调用 `Reconciler` 接口的 `Reconcile(ctx context.Context,key string) err` 函数

4. `Reconcile(ctx context.Context,key string) err` 函数调用 具体的 Reconciler 的实现接口 (**这里就是用户自己实现的代码了**)`sample-controller/pkg/client/injection/reconciler/samples/v1alpha1/addressableservice/reconciler.go#181`

   * `ReconcileKind(ctx context.Context, o v1alpha1.AddressableService) reconciler.Event`

   - `FinalizeKind(ctx context.Context, o v1alpha1.AddressableService) reconciler.Event`

5. 接下来就是上述第 4点说的自己实现的代码了



## Controller逻辑

创建`pkg/reconciler/mydeployment/controller.go`文件，定义`NewController`方法，如下：

```go
import (
	"context"
	"knative.dev/pkg/configmap"
	knativeController "knative.dev/pkg/controller"
)
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *knativeController.Impl {

	return nil;
}
```



此时就需要考虑controller的具体逻辑了。

这里就简单地维护pod数量为例：要能查看MyDeployment对应的pods数量，如果pods数量少于或多余`Replicas`指定的数量，能够自动创建或删除pod。

因此，我们封装如下的一个结构体：

```go
type Reconciler struct {
	// 能够列出当前pod的接口
	PodLister k8slisters.PodLister
	// k8s API, 能够增删改查api
	kubeClient kubernetes.Interface
}
```



前面第4点提到Reconcile函数的调用，为此我们要实现ReconcileKind方法的逻辑，这里直接去实现`mydeploymentReconciler.Interface`接口，由idea来为我们自动不全代码：

```go
var (
	_ mydeploymentReconciler.Interface = (*Reconciler)(nil)
)

func (r Reconciler) ReconcileKind(ctx context.Context, o *v1.MyDeployment) knativeReconciler.Event {

	logger := logging.FromContext(ctx)

	ns := o.Namespace
    // 获取当前存在的pods
	podList, err := r.PodLister.Pods(ns).List(labels.SelectorFromSet(labels.Set{}))
	if err!=nil {
		return fmt.Errorf("failed to list existing pods: %w", err)
	}
	logger.Infof("Found %d pods in total", len(podList))
	replicas, err := strconv.Atoi(o.Spec.Replicas)
	toCreate := replicas - len(podList)
	logger.Infof("Got %d existing pods, desired  Replicas is %d",len(podList),replicas)
	if toCreate>0{
		// 需要创建pod
		logger.Infof("need create %d pods",toCreate)
		pods := makePods(o)
		for i := 0; i < toCreate; i++ {
			_, err := r.kubeClient.CoreV1().Pods(pods.Namespace).Create(ctx, pods, metav1.CreateOptions{})
			if err!=nil {
				return fmt.Errorf("failed to create pod: %w", err)
			}
		}
	}else if toCreate<0 {
		//多余的pod， 需要删除
		toDelete := toCreate * -1
		logger.Infof("need delete %d pods",toDelete)
		for i := 0; i < toDelete; i++ {
			r.kubeClient.CoreV1().Pods(o.Namespace).Delete(ctx,podList[i].Name,metav1.DeleteOptions{})
		}
	}


	return nil
}

func makePods(d *v1.MyDeployment) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ObjectMeta.Namespace,
			GenerateName: d.Name+"-",
			Labels: map[string]string{
				// The label allows for easy querying of all the pods created.
				demo.GroupName+"/podOwner": d.Name,
			},
			// The OwnerReference makes sure the pods get removed automatically once the
			// SimpleDeployment is removed.
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(d)},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "abreaking-container",Image: d.Spec.Image},
			},
		},
	}
}
```



`ReconcileKind`方法里就是前面我们说的逻辑：

1. 首先通过`r.PodLister.Pods(ns).List`获取到当前namespace里存在的pods；
2. 判断存在的pods数量与指定的Replicas是否多了或者少了；
3. 如果少了，就再创建pod；多了就删除多余的pod；
4. 创建pod的方法是直接使用了`kubeClient`提供的api，在`makePods`指定要创建的pod有哪些属性。



然后再到`NewController`方法里指定实例化`Reconciler`的逻辑：

```

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
```

handle函数：为 `informer` 添加 函数除了实例中的  `Informer().AddEventHandler`，还可以 通过 `Informer().AddEventHandlerWithResyncPeriod` 确保除了 `watch` 之外，周期性将 `CR` 全量加入 工作队列中处理。

filter 函数： 还可以添加如下 filter 函数，过滤进入 工作队列的 资源，(在资源数量巨大时能优化性能)。



# 部署调试

## 生成CRD描述文件

1. 在`cmd/schema/main.go`注册`MyDeployment`：

   ```
   import (
   	v1 "knative.dev/sample-controller/pkg/apis/demo/v1"
   	"log"
   
   	"knative.dev/hack/schema/commands"
   	"knative.dev/hack/schema/registry"
   )
   
   // schema is a tool to dump the schema for Eventing resources.
   func main() {
   	registry.Register(&v1.MyDeployment{})
   
   	if err := commands.New("knative.dev/sample-controller").Execute(); err != nil {
   		log.Fatal("Error during command execution: ", err)
   	}
   }
   ```

2. 而后执行命令如下命令来生成crd描述文件：

   ```shell
   go run cmd/schema/main.go dump MyDeployment
   ```

   生成的内容比较长，大概如下：

   ```yaml
   type: object
   properties:
     spec:
       type: object
       properties:
       	...
     status:
       type: object
       properties:
         annotations:
         	...
         conditions:
         	...
         observedGeneration:
          	...
         readyReplicas:
         	...
   ```



3. 仿照原来的300-*.yaml文件，创建`300-mydeployment.yaml`，内容如下：

   ```yaml
   apiVersion: apiextensions.k8s.io/v1
   kind: CustomResourceDefinition
   metadata:
     name: mydeployments.demo.abreaking.com
     labels:
       samples.knative.dev/release: devel
       knative.dev/crd-install: "true"
   spec:
     group: demo.abreaking.com
     versions:
       - name: v1
         served: true
         storage: true
         subresources:
           status: { }
         schema:
           openAPIV3Schema:
           	# 将上面生成的内容粘贴到这里
     names:
       kind: MyDeployment
       plural: mydeployments
       singular: mydpm
       categories:
       - all
       - knative
       shortNames:
       - sdeploy
     scope: Namespaced
   
   ```

   然后将将上面生成的内容粘贴到`spec.version.schema.openAPIV3Schema`下面即可。

4. 最后直接通过如下命令创建crd：

   ```shell
   k apply -f config/300-mydeployment.yaml
   ```

   （k = kubectl ，这里简写了，下同）

5. 可以通过如下命令来进行验证创建的crd：

   ```shell
   # abreaking是我group里的关键字
   $ k get crd | grep abreaking
   mydeployments.demo.abreaking.com       2022-04-12T04:20:44Z
   ```



## 构建部署

直接通过`ko`来进行构建和部署，这里使用ko来进行工程的构建和部署，如果还不会，可以参考这篇文章：[使用ko在Kubernetes上构建和部署go应用 (abreaking.com)：https://blog.abreaking.com/article/162](https://blog.abreaking.com/article/162)。

构建文件为：`config/controller.yaml`，通过如下命令进行构建和部署：

```shell
ko apply -f config/controller.yaml
```



## 测试

然后我们就可以来进行测试我们编写的控制器逻辑。

首先，创建一件简单的部署文件`example-mydeployment.yaml`，内容如下：

```yaml
apiVersion: demo.abreaking.com/v1
kind: MyDeployment
metadata:
   name: example-my-deployment
spec:
   image: abreaking/helloworld-java
   replicas: "3"

```



然后，通过如下命令直接部署：

```shell
k apply -f example-mydeployment.yaml
```

此时查看pod，可以看到，数量与`replicas`一致：

```shell
$ k get pods
NAME                          READY   STATUS              RESTARTS   AGE
example-my-deployment-8mm2s   0/1     ContainerCreating   0          78s
example-my-deployment-wqs5h   0/1     ContainerCreating   0          12s
example-my-deployment-xjhpm   1/1     Running             0          98s
```

等全部都是running时候，删除其中一个pod：

```shell
$ k delete pod example-my-deployment-8mm2s
pod "example-my-deployment-8mm2s" deleted
```

再次观察，此时发现新的pod正在创建：

```shell
$ k get pods
NAME                          READY   STATUS              RESTARTS   AGE
example-my-deployment-54bmt   0/1     ContainerCreating   0          22s
example-my-deployment-wqs5h   1/1     Running             0          18m
example-my-deployment-xjhpm   1/1     Running             0          19m
```



## 调试debug

如果需要在本地开发环境上的ide上进行debug调试，可以参考：[kubernetes本地开发环境搭建 (abreaking.com)：https://blog.abreaking.com/article/165](https://blog.abreaking.com/article/165)



# 代码位置

本文源码地址：[https://github.com/aBreaking/knative-sample-controller.git](https://github.com/aBreaking/knative-sample-controller.git)

knative controller开发模板代码地址：[https://github.com/knative-sandbox/sample-controller](https://github.com/knative-sandbox/sample-controller)

# 参考资料：

[如何从零开始编写一个Kubernetes CRD · Service Mesh](https://www.servicemesher.com/blog/kubernetes-crd-quick-start/)

[Extend the Kubernetes API with CustomResourceDefinitions | Kubernetes](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)

[OperatorHub.io | The registry for Kubernetes Operators](https://operatorhub.io/)

[如何基于 Knative 开发 自定义controller - knative 指南](https://knative.club/advanced-development/custom-controller#2.4-kong-zhi-qi-luo-ji-jie-shao)

