package k8s

//go:generate popeye gen

import (
	"github.com/derailed/popeye/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
)

var (
	supportedMetricsAPIVersions = []string{"v1beta1"}
	systemNS                    = []string{"kube-system", "kube-public"}
)

// Client represents a Kubernetes api server client.
type Client struct {
	Config *config.Config

	api        kubernetes.Interface
	pods       []v1.Pod
	namespaces []v1.Namespace
}

// NewClient returns a dialable api server configuration.
func NewClient(config *config.Config) *Client {
	return &Client{Config: config}
}

// DialOrDie returns an api server client connection or dies.
func (c *Client) DialOrDie() kubernetes.Interface {
	client, err := c.Dial()
	if err != nil {
		panic(err)
	}
	return client
}

// Dial returns a handle to api server.
func (c *Client) Dial() (kubernetes.Interface, error) {
	if c.api != nil {
		return c.api, nil
	}

	cfg, err := c.Config.RESTConfig()
	if err != nil {
		return nil, err
	}

	if c.api, err = kubernetes.NewForConfig(cfg); err != nil {
		return nil, err
	}
	return c.api, nil
}

// ClusterHasMetrics checks if metrics server is available on the cluster.
func (c *Client) ClusterHasMetrics() bool {
	srv, err := c.Dial()
	if err != nil {
		return false
	}
	apiGroups, err := srv.Discovery().ServerGroups()
	if err != nil {
		return false
	}

	for _, discoveredAPIGroup := range apiGroups.Groups {
		if discoveredAPIGroup.Name != metricsapi.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			for _, supportedVersion := range supportedMetricsAPIVersions {
				if version.Version == supportedVersion {
					return true
				}
			}
		}
	}
	return false
}

// ListPods list all available pods.
func (c *Client) ListPods() ([]v1.Pod, error) {
	if len(c.pods) != 0 {
		return c.pods, nil
	}

	ns := c.Config.ActiveNamespace()
	ll, err := c.DialOrDie().CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	c.pods = make([]v1.Pod, 0, len(ll.Items))
	for _, po := range ll.Items {
		if c.Config.ExcludedNS(po.Namespace) {
			continue
		}
		c.pods = append(c.pods, po)
	}

	return c.pods, nil
}

// ListNS lists all available namespaces.
func (c *Client) ListNS() ([]v1.Namespace, error) {
	if len(c.namespaces) != 0 {
		return c.namespaces, nil
	}

	ns := c.Config.ActiveNamespace()
	var nn *v1.NamespaceList
	var err error
	if ns == "" {
		nn, err = c.DialOrDie().CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		n, err := c.DialOrDie().CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		nn = &v1.NamespaceList{Items: []v1.Namespace{*n}}
	}

	c.namespaces = make([]v1.Namespace, 0, len(nn.Items))
	for _, ns := range nn.Items {
		if c.Config.ExcludedNS(ns.Name) {
			continue
		}
		c.namespaces = append(c.namespaces, ns)
	}

	return c.namespaces, nil
}

func isSystemNS(ns string) bool {
	for _, n := range systemNS {
		if n == ns {
			return true
		}
	}
	return false
}