package bareinfra

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


// Provider implements the virtual-kubelet provider interface and communicates with the Nomad API.
type Provider struct {
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	pods            []*v1.Pod
}

// NewProvider creates a new Provider
func NewProvider(rm *manager.ResourceManager, nodeName, operatingSystem  string) (*Provider, error) {
	p := Provider{}
	p.resourceManager = rm
	p.nodeName = nodeName
	p.operatingSystem = operatingSystem

	return &p, nil
}

// CreatePod accepts a Pod definition and creates
// a Nomad job
func (p *Provider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("CreatePod %q\n", pod.Name)
	_, err := p.GetPod(ctx, pod.Namespace, pod.Name)
	if err == nil {
		return fmt.Errorf("Pod: \"%s\" in namespace \"%s\" exist", pod.Name, pod.Namespace)
	}
	podIP := pod.Annotations["vk/PodIP"]
	
	containerStatusesMap := []v1.ContainerStatus{}
	
	for _, c := range pod.Spec.Containers {
		cs := v1.ContainerStatus{
			Name: c.Name,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: metav1.Now(),
				},
			},
			Ready: true,
			RestartCount: 0,
		}
		containerStatusesMap = append(containerStatusesMap, cs)
	}

	pod.Status = v1.PodStatus{
		PodIP: podIP,
		Phase: v1.PodRunning,
		Conditions: []v1.PodCondition{v1.PodCondition{
            Type:   v1.PodReady,
            Status: v1.ConditionTrue,
        }},
		ContainerStatuses: containerStatusesMap,

	}
	p.pods = append(p.pods, pod)
	return nil
}

// UpdatePod is a noop, nomad does not support live updates of a pod.
func (p *Provider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.Println("Pod Update called: No-op as not implemented")
	return nil
}

// DeletePod accepts a Pod definition and deletes a Nomad job.
func (p *Provider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	// Deregister job
	namespace := pod.Namespace
	name := pod.Name
	pods := p.pods
	for index, podI := range pods {
        if podI.Name == name && podI.Namespace == namespace {
			p.pods = append(pods[:index], pods[index+1:]...)
			break
        }
    }

	return nil
}

// GetPod returns the pod running in the Nomad cluster. returns nil
// if pod is not found.
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	for _, podI := range p.pods {
		if podI.Name == name && podI.Namespace == namespace {
			return podI, nil
		}
	}
	return pod, fmt.Errorf("Pod: \"%s\" in namespace \"%s\" not found", name, namespace)
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *Provider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("")), nil
}

// GetPodFullName as defined in the provider context
func (p *Provider) GetPodFullName(ctx context.Context, namespace string, pod string) string {
	return fmt.Sprintf("%s-%s", namespace, pod)
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *Provider) RunInContainer(ctx context.Context, namespace, name, container string, cmd []string, attach api.AttachIO) error {
	log.Printf("ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is running as a job
// in the Nomad cluster returns nil if a pod by that name is not found.
func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return &v1.PodStatus{}, err
	}
	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be running in Nomad nodes.
func (p *Provider) GetPods(ctx context.Context) ([]*v1.Pod, error) {

	return p.pods, nil
}

// Capacity returns a resource list containing the capacity limits set for Nomad.
func (p *Provider) Capacity(ctx context.Context) v1.ResourceList {
	// TODO: Use nomad /nodes api to get a list of nodes in the cluster
	// and then use the read node /node/:node_id endpoint to calculate
	// the total resources of the cluster to report back to kubernetes.
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *Provider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make these dynamic.
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}

}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *Provider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	// TODO: Use nomad api to get a list of node addresses.
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *Provider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *Provider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}
