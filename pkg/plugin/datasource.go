package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
)

// HelmQuery represents the expected structure of JSON input.
type HelmQuery struct {
	Namespace   string `json:"namespace"`
	ReleaseName string `json:"release"`
}
// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
type Datasource struct{}

// Resource represents a Kubernetes resource.
type Resource struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// getKubernetesClient initializes a Kubernetes client.
func getKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	return clientset, nil
}

// getHelmResources fetches all resources associated with a Helm release.
func getHelmResources(namespace, releaseName string) ([]Resource, error) {
	clientset, err := getKubernetesClient()
	if err != nil {
		return nil, err
	}

	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", releaseName)
	var resources []Resource

	// Fetch Pods
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	for _, pod := range pods.Items {
		resources = append(resources, Resource{"Pod", pod.Name, string(pod.Status.Phase)})
	}

	// Fetch Services
	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	for _, service := range services.Items {
		resources = append(resources, Resource{"Service", service.Name, ""})
	}

	// Fetch Deployments
	deployments, err := clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	for _, deployment := range deployments.Items {
		resources = append(resources, Resource{"Deployment", deployment.Name, ""})
	}

	return resources, nil
}

// QueryData handles requests from Grafana.
func (ds *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()

	for _, query := range req.Queries {
		queryData := backend.DataResponse{}

		// Step 1: Unmarshal JSON query data
		var helmQuery HelmQuery
		err := json.Unmarshal(query.JSON, &helmQuery)
		if err != nil {
			queryData.Error = fmt.Errorf("failed to parse query JSON: %v", err)
			response.Responses[query.RefID] = queryData
			continue
		}

		// Step 2: Validate extracted fields
		if helmQuery.Namespace == "" {
			queryData.Error = fmt.Errorf("missing or invalid 'namespace'")
			response.Responses[query.RefID] = queryData
			continue
		}
		if helmQuery.ReleaseName == "" {
			queryData.Error = fmt.Errorf("missing or invalid 'release'")
			response.Responses[query.RefID] = queryData
			continue
		}

		// Step 3: Fetch resources from Kubernetes
		resources, err := getHelmResources(helmQuery.Namespace, helmQuery.ReleaseName)
		if err != nil {
			queryData.Error = err
		} else {
			queryData.Error = nil
			// Step 4: Use data.NewFrame for structured response
			frame := data.NewFrame("response",
				data.NewField("kind", nil, []string{}),
				data.NewField("name", nil, []string{}),
				data.NewField("status", nil, []string{}),
			)

			for _, resource := range resources {
				frame.AppendRow(resource.Kind, resource.Name, resource.Status)
			}

			queryData.Frames = append(queryData.Frames, frame)
		}

		response.Responses[query.RefID] = queryData
	}

	return response, nil
}











// NewDatasource creates a new instance of the datasource.
func NewDatasource(_ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{}, nil 
}
