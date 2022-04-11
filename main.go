package main


import (
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"reflect"

	userclientset "k8s-filter/pkg/client/clientset/versioned"
	userinformers "k8s-filter/pkg/client/informers/externalversions"
)

func NewConfig() *rest.Config {
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
	return config
}

const defaultResync = 600 * time.Second

func main()  {
	config := NewConfig()
	kubeClient := kubernetes.NewForConfigOrDie(config)
	userClient := userclientset.NewForConfigOrDie(config)

	kubeInformers := informers.NewSharedInformerFactory(kubeClient, defaultResync)
	userInformers := userinformers.NewSharedInformerFactory(userClient, defaultResync)

	var ch <-chan struct{}


	k8sGVRs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "", Version: "v1", Resource: "nodes"},
		{Group: "", Version: "v1", Resource: "resourcequotas"},
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "configmaps"},

		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},

		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "controllerrevisions"},

		{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},

		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1beta1", Resource: "cronjobs"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	}

	for _, gvr := range k8sGVRs {
		_, err := kubeInformers.ForResource(gvr)
		if err != nil {
			klog.Errorf("cannot create informer for %s", gvr)
		}
	}
	
	_, err := userInformers.ForResource(schema.GroupVersionResource{
		Group:    "stable.example.com",
		Version:  "v1",
		Resource: "users",
	})
	if err != nil{
		klog.Error("err user")
	}

	kubeInformers.Start(ch)
	kubeInformers.WaitForCacheSync(ch)

	userInformers.Start(ch)
	userInformers.WaitForCacheSync(ch)

	nodes, _ := kubeInformers.Core().V1().Nodes().Lister().List(labels.Everything())
	deploys, _ := kubeInformers.Apps().V1().Deployments().Lister().List(labels.Everything())

	// crd
	users, _ := userInformers.Stable().V1().Users().Lister().List(labels.Everything())
	for _, u := range users{
		fmt.Println(u)
	}


	var t []interface{}
	for _, d := range nodes{
		t = append(t, d)
	}
	result := Filter(t, "projectcalico.org/kube-labels", true, "aaaa")
	fmt.Println(result)
	fmt.Println(len(result))


	var dep []interface{}
	for _, d := range deploys{
		dep = append(dep, d)
	}
	rst2 := Filter(dep,"kubernetes.io/created-by", true)
	fmt.Println(rst2)
	fmt.Println(len(rst2))


	var us []interface{}
	for _, u := range users{
		us = append(us, u)
	}
	rst3 := Filter(us, "kubectl.kubernetes.io/last-applied-configuration", true)
	for _, u := range rst3{
		fmt.Println(u)
	}
	fmt.Println(len(rst3))
}

func Filter(items []interface{}, key string, get bool, value ...string)(result []interface{}){
	for _, item := range items{
		valueOfItem := reflect.ValueOf(item)
		annotations := valueOfItem.Elem().FieldByName("ObjectMeta").FieldByName("Annotations")
		filter := annotations.Interface().(map[string]string)[key]
		if len(value) == 0{
			if get{
				if filter != ""{
					result = append(result, item)
				}
				continue
			}
			if filter == ""{
				result = append(result, item)
			}
			continue
		}
		if len(value) != 0{
			if get{
				if filter == value[0] {
					result = append(result, item)
				}
				continue
			}
			if filter != value[0]{
				result = append(result, item)
			}
		}
	}
	return result
}