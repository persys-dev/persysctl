package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/persys-dev/persysctl/internal/auth"
	"github.com/persys-dev/persysctl/internal/config"
	controlv1 "github.com/persys-dev/persysctl/internal/controlv1"
	"github.com/persys-dev/persysctl/internal/models"
	agentv1 "github.com/persys/compute-agent/pkg/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	cfg             config.Config
	httpClient      *http.Client
	grpcConn        *grpc.ClientConn
	schedulerClient controlv1.AgentControlClient
	agentClient     agentv1.AgentServiceClient
}

type ScheduleResponse struct {
	WorkloadID string `json:"workloadId"`
	NodeID     string `json:"nodeId"`
	Status     string `json:"status"`
}

func NewClient(cfg config.Config) (*Client, error) {
	c := &Client{cfg: cfg}

	switch cfg.Transport {
	case "http":
		httpClient, err := newHTTPClient(cfg)
		if err != nil {
			return nil, err
		}
		c.httpClient = httpClient
	case "grpc":
		conn, schedulerClient, agentClient, err := newGRPCClient(cfg)
		if err != nil {
			return nil, err
		}
		c.grpcConn = conn
		c.schedulerClient = schedulerClient
		c.agentClient = agentClient
	default:
		return nil, fmt.Errorf("unsupported transport %q (expected http or grpc)", cfg.Transport)
	}

	return c, nil
}

func (c *Client) Close() error {
	if c.grpcConn != nil {
		return c.grpcConn.Close()
	}
	return nil
}

func newHTTPClient(cfg config.Config) (*http.Client, error) {
	if cfg.APIEndpoint == "" {
		return nil, fmt.Errorf("api_endpoint is required for http transport")
	}
	if strings.HasPrefix(cfg.APIEndpoint, "http://") {
		return &http.Client{}, nil
	}
	if !strings.HasPrefix(cfg.APIEndpoint, "https://") {
		return nil, fmt.Errorf("api_endpoint must start with http:// or https://, got: %s", cfg.APIEndpoint)
	}

	certMgr := auth.NewCertificateManager(
		cfg.CACertPath,
		cfg.CertPath,
		cfg.KeyPath,
		cfg.CFSSLApiURL,
		cfg.CommonName,
		cfg.Organization,
	)
	if err := certMgr.EnsureCertificate(); err != nil {
		return nil, fmt.Errorf("failed to ensure certificate: %w", err)
	}
	tlsConfig, err := certMgr.GetTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS config: %w", err)
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}

func newGRPCClient(cfg config.Config) (*grpc.ClientConn, controlv1.AgentControlClient, agentv1.AgentServiceClient, error) {
	if cfg.GRPCEndpoint == "" {
		return nil, nil, nil, fmt.Errorf("grpc_endpoint is required for grpc transport")
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.RPCTimeoutSeconds)*time.Second)
	defer cancel()

	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if cfg.GRPCInsecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, err := buildMTLSConfig(cfg)
		if err != nil {
			return nil, nil, nil, err
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}

	conn, err := grpc.DialContext(dialCtx, cfg.GRPCEndpoint, dialOpts...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to gRPC endpoint %s: %w", cfg.GRPCEndpoint, err)
	}

	return conn, controlv1.NewAgentControlClient(conn), agentv1.NewAgentServiceClient(conn), nil
}

func buildMTLSConfig(cfg config.Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate/key: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
	}, nil
}

func (c *Client) rpcContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(c.cfg.RPCTimeoutSeconds)*time.Second)
}

func (c *Client) requireSchedulerGRPC() error {
	if c.cfg.Transport != "grpc" {
		return fmt.Errorf("this operation requires gRPC transport (set --transport grpc)")
	}
	if c.cfg.GRPCTarget != "scheduler" {
		return fmt.Errorf("this operation requires scheduler target (set --grpc-target scheduler)")
	}
	return nil
}

func (c *Client) requireAgentGRPC() error {
	if c.cfg.Transport != "grpc" {
		return fmt.Errorf("this operation requires gRPC transport (set --transport grpc)")
	}
	if c.cfg.GRPCTarget != "agent" {
		return fmt.Errorf("this operation requires compute-agent target (set --grpc-target agent)")
	}
	return nil
}

func (c *Client) ScheduleWorkload(workload models.Workload) (*ScheduleResponse, error) {
	switch c.cfg.Transport {
	case "grpc":
		return c.scheduleWorkloadGRPC(workload)
	case "http":
		return c.scheduleWorkloadHTTP(workload)
	default:
		return nil, fmt.Errorf("unsupported transport %q", c.cfg.Transport)
	}
}

func (c *Client) scheduleWorkloadGRPC(workload models.Workload) (*ScheduleResponse, error) {
	ctx, cancel := c.rpcContext()
	defer cancel()

	switch c.cfg.GRPCTarget {
	case "scheduler":
		req, workloadID, err := toSchedulerApplyRequest(workload)
		if err != nil {
			return nil, err
		}
		resp, err := c.schedulerClient.ApplyWorkload(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("scheduler apply workload failed: %w", err)
		}
		status := "applied"
		if !resp.GetSuccess() {
			status = "failed"
		}
		return &ScheduleResponse{WorkloadID: workloadID, NodeID: "scheduler-managed", Status: status}, nil
	case "agent":
		req, workloadID, err := toAgentApplyRequest(workload)
		if err != nil {
			return nil, err
		}
		resp, err := c.agentClient.ApplyWorkload(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("compute-agent apply workload failed: %w", err)
		}
		status := "applied"
		if !resp.GetApplied() {
			status = "failed"
		}
		return &ScheduleResponse{WorkloadID: workloadID, NodeID: "standalone-agent", Status: status}, nil
	default:
		return nil, fmt.Errorf("unsupported grpc_target %q (expected scheduler or agent)", c.cfg.GRPCTarget)
	}
}

func (c *Client) ListWorkloads(nodeID, status string) ([]models.Workload, error) {
	switch c.cfg.Transport {
	case "http":
		return c.listWorkloadsHTTP()
	case "grpc":
		ctx, cancel := c.rpcContext()
		defer cancel()

		if c.cfg.GRPCTarget == "scheduler" {
			resp, err := c.schedulerClient.ListWorkloads(ctx, &controlv1.ListWorkloadsRequest{NodeId: nodeID, Status: status})
			if err != nil {
				return nil, fmt.Errorf("scheduler list workloads failed: %w", err)
			}
			out := make([]models.Workload, 0, len(resp.GetWorkloads()))
			for _, w := range resp.GetWorkloads() {
				out = append(out, models.Workload{
					ID:     w.GetWorkloadId(),
					Type:   w.GetType(),
					NodeID: w.GetAssignedNodeId(),
					Status: w.GetStatus(),
				})
			}
			return out, nil
		}

		resp, err := c.agentClient.ListWorkloads(ctx, &agentv1.ListWorkloadsRequest{})
		if err != nil {
			return nil, fmt.Errorf("compute-agent list workloads failed: %w", err)
		}
		out := make([]models.Workload, 0, len(resp.GetWorkloads()))
		for _, w := range resp.GetWorkloads() {
			out = append(out, models.Workload{
				ID:     w.GetId(),
				Type:   workloadTypeToString(w.GetType()),
				Status: strings.ToLower(strings.TrimPrefix(w.GetActualState().String(), "ACTUAL_STATE_")),
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported transport %q", c.cfg.Transport)
	}
}

func (c *Client) GetWorkload(workloadID string) (*controlv1.GetWorkloadResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.GetWorkload(ctx, &controlv1.GetWorkloadRequest{WorkloadId: workloadID})
}

func (c *Client) DeleteWorkload(workloadID string) (*controlv1.DeleteWorkloadResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.DeleteWorkload(ctx, &controlv1.DeleteWorkloadRequest{WorkloadId: workloadID})
}

func (c *Client) RetryWorkload(workloadID string) (*controlv1.RetryWorkloadResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.RetryWorkload(ctx, &controlv1.RetryWorkloadRequest{WorkloadId: workloadID})
}

func (c *Client) ListNodes(status string) ([]models.Node, error) {
	switch c.cfg.Transport {
	case "http":
		return c.listNodesHTTP()
	case "grpc":
		if err := c.requireSchedulerGRPC(); err != nil {
			return nil, err
		}
		ctx, cancel := c.rpcContext()
		defer cancel()
		resp, err := c.schedulerClient.ListNodes(ctx, &controlv1.ListNodesRequest{Status: status})
		if err != nil {
			return nil, fmt.Errorf("scheduler list nodes failed: %w", err)
		}

		nodes := make([]models.Node, 0, len(resp.GetNodes()))
		for _, n := range resp.GetNodes() {
			nodes = append(nodes, models.Node{
				NodeID:    n.GetNodeId(),
				IPAddress: n.GetGrpcEndpoint(),
				Status:    n.GetStatus(),
				Resources: models.Resources{
					CPU:    int(math.Round(n.GetTotalCpuCores() * 1000)),
					Memory: int(n.GetTotalMemoryMb()),
				},
				Labels: n.GetLabels(),
			})
		}
		return nodes, nil
	default:
		return nil, fmt.Errorf("unsupported transport %q", c.cfg.Transport)
	}
}

func (c *Client) GetNode(nodeID string) (*controlv1.GetNodeResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.GetNode(ctx, &controlv1.GetNodeRequest{NodeId: nodeID})
}

func (c *Client) SchedulerListNodes(status string) (*controlv1.ListNodesResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.ListNodes(ctx, &controlv1.ListNodesRequest{Status: status})
}

func (c *Client) SchedulerListWorkloads(nodeID, status string) (*controlv1.ListWorkloadsResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.ListWorkloads(ctx, &controlv1.ListWorkloadsRequest{NodeId: nodeID, Status: status})
}

func (c *Client) GetClusterSummary() (*controlv1.GetClusterSummaryResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.GetClusterSummary(ctx, &controlv1.GetClusterSummaryRequest{})
}

func (c *Client) RegisterNode(req *controlv1.RegisterNodeRequest) (*controlv1.RegisterNodeResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.RegisterNode(ctx, req)
}

func (c *Client) Heartbeat(req *controlv1.HeartbeatRequest) (*controlv1.HeartbeatResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.Heartbeat(ctx, req)
}

func (c *Client) ControlStreamSend(msg *controlv1.ControlMessage) (*controlv1.ControlMessage, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()

	stream, err := c.schedulerClient.ControlStream(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(msg); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) ApplySchedulerWorkload(req *controlv1.ApplyWorkloadRequest) (*controlv1.ApplyWorkloadResponse, error) {
	if err := c.requireSchedulerGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.schedulerClient.ApplyWorkload(ctx, req)
}

func (c *Client) ApplyAgentWorkload(req *agentv1.ApplyWorkloadRequest) (*agentv1.ApplyWorkloadResponse, error) {
	if err := c.requireAgentGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.agentClient.ApplyWorkload(ctx, req)
}

func (c *Client) AgentHealthCheck() (*agentv1.HealthCheckResponse, error) {
	if err := c.requireAgentGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.agentClient.HealthCheck(ctx, &agentv1.HealthCheckRequest{})
}

func (c *Client) AgentGetWorkloadStatus(workloadID string) (*agentv1.GetWorkloadStatusResponse, error) {
	if err := c.requireAgentGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.agentClient.GetWorkloadStatus(ctx, &agentv1.GetWorkloadStatusRequest{Id: workloadID})
}

func (c *Client) AgentDeleteWorkload(workloadID string) (*agentv1.DeleteWorkloadResponse, error) {
	if err := c.requireAgentGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.agentClient.DeleteWorkload(ctx, &agentv1.DeleteWorkloadRequest{Id: workloadID})
}

func (c *Client) AgentListActions(workloadID, actionType, status string, limit int32, newestFirst bool) (*agentv1.ListActionsResponse, error) {
	if err := c.requireAgentGRPC(); err != nil {
		return nil, err
	}
	ctx, cancel := c.rpcContext()
	defer cancel()
	return c.agentClient.ListActions(ctx, &agentv1.ListActionsRequest{
		WorkloadId:  workloadID,
		ActionType:  actionType,
		Status:      status,
		Limit:       limit,
		NewestFirst: newestFirst,
	})
}

func (c *Client) GetMetrics() (map[string]interface{}, error) {
	if c.cfg.Transport == "grpc" && c.cfg.GRPCTarget == "scheduler" {
		summary, err := c.GetClusterSummary()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"totalNodes":       summary.GetTotalNodes(),
			"readyNodes":       summary.GetReadyNodes(),
			"notReadyNodes":    summary.GetNotReadyNodes(),
			"totalWorkloads":   summary.GetTotalWorkloads(),
			"runningWorkloads": summary.GetRunningWorkloads(),
			"pendingWorkloads": summary.GetPendingWorkloads(),
			"failedWorkloads":  summary.GetFailedWorkloads(),
			"deletedWorkloads": summary.GetDeletedWorkloads(),
			"generatedAt":      summary.GetGeneratedAt().AsTime().Format(time.RFC3339),
		}, nil
	}
	if c.cfg.Transport != "http" {
		return nil, fmt.Errorf("metrics are available only with scheduler gRPC or http transport")
	}

	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/cluster/metrics", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return metrics, nil
}

func (c *Client) scheduleWorkloadHTTP(workload models.Workload) (*ScheduleResponse, error) {
	payload, err := json.Marshal(workload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workload: %v", err)
	}

	resp, err := c.makeRequest("POST", "/workloads/schedule", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var scheduleResp ScheduleResponse
	if err := json.Unmarshal(body, &scheduleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return &scheduleResp, nil
}

func (c *Client) listWorkloadsHTTP() ([]models.Workload, error) {
	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/workloads", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var workloads []models.Workload
	if err := json.Unmarshal(body, &workloads); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return workloads, nil
}

func (c *Client) listNodesHTTP() ([]models.Node, error) {
	req, err := http.NewRequest("GET", c.cfg.APIEndpoint+"/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Nodes []models.Node `json:"nodes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return result.Nodes, nil
}

func (c *Client) makeRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.cfg.APIEndpoint + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	return resp, nil
}

func workloadID(w models.Workload) string {
	if w.ID != "" {
		return w.ID
	}
	if w.Name != "" {
		return w.Name
	}
	return fmt.Sprintf("workload-%d", time.Now().Unix())
}

func toSchedulerApplyRequest(w models.Workload) (*controlv1.ApplyWorkloadRequest, string, error) {
	id := workloadID(w)
	spec, err := toSchedulerWorkloadSpec(w)
	if err != nil {
		return nil, "", err
	}

	revision := fmt.Sprintf("rev-%d", time.Now().Unix())
	if w.ID != "" {
		revision = w.ID + "-rev"
	}

	return &controlv1.ApplyWorkloadRequest{
		WorkloadId:   id,
		RevisionId:   revision,
		DesiredState: "Running",
		Spec:         spec,
	}, id, nil
}

func toSchedulerWorkloadSpec(w models.Workload) (*controlv1.WorkloadSpec, error) {
	spec := &controlv1.WorkloadSpec{
		Resources: &controlv1.ResourceRequirements{
			CpuMillicores: int64(w.Resources.CPU),
			MemoryMb:      int64(w.Resources.Memory),
		},
		Metadata: w.Labels,
	}

	switch w.Type {
	case "docker-container", "container":
		spec.Type = "container"
		spec.Workload = &controlv1.WorkloadSpec_Container{Container: &controlv1.ContainerSpec{
			Image:         w.Image,
			Command:       parseCommand(w.Command),
			Env:           w.EnvVars,
			Volumes:       parseControlVolumes(w.Volumes),
			Ports:         parseControlPorts(w.Ports),
			RestartPolicy: w.RestartPolicy,
		}}
	case "docker-compose", "git-compose", "compose":
		rawCompose, _, err := resolveCompose(w)
		if err != nil {
			return nil, err
		}
		spec.Type = "compose"
		compose := &controlv1.ComposeSpec{Env: w.EnvVars}
		if w.Type == "git-compose" && w.GitRepo != "" {
			compose.SourceType = "git"
			compose.GitRepo = w.GitRepo
			compose.GitRef = w.GitBranch
		} else {
			compose.SourceType = "inline"
			compose.InlineYaml = rawCompose
		}
		spec.Workload = &controlv1.WorkloadSpec_Compose{Compose: compose}
	default:
		return nil, fmt.Errorf("unsupported workload type for scheduler grpc: %s", w.Type)
	}

	return spec, nil
}

func toAgentApplyRequest(w models.Workload) (*agentv1.ApplyWorkloadRequest, string, error) {
	id := workloadID(w)
	req := &agentv1.ApplyWorkloadRequest{
		Id:           id,
		RevisionId:   fmt.Sprintf("rev-%d", time.Now().Unix()),
		DesiredState: agentv1.DesiredState_DESIRED_STATE_RUNNING,
		Spec:         &agentv1.WorkloadSpec{},
	}

	switch w.Type {
	case "docker-container", "container":
		req.Type = agentv1.WorkloadType_WORKLOAD_TYPE_CONTAINER
		req.Spec.Spec = &agentv1.WorkloadSpec_Container{Container: &agentv1.ContainerSpec{
			Image:   w.Image,
			Command: parseCommand(w.Command),
			Env:     w.EnvVars,
			Volumes: parseAgentVolumes(w.Volumes),
			Ports:   parseAgentPorts(w.Ports),
			RestartPolicy: &agentv1.RestartPolicy{
				Policy: w.RestartPolicy,
			},
			Labels: w.Labels,
		}}
	case "docker-compose", "compose":
		_, base64Compose, err := resolveCompose(w)
		if err != nil {
			return nil, "", err
		}
		req.Type = agentv1.WorkloadType_WORKLOAD_TYPE_COMPOSE
		req.Spec.Spec = &agentv1.WorkloadSpec_Compose{Compose: &agentv1.ComposeSpec{ProjectName: w.Name, ComposeYaml: base64Compose, Env: w.EnvVars}}
	case "git-compose":
		return nil, "", fmt.Errorf("git-compose is not supported in standalone compute-agent mode; use docker-compose with inline/local compose content")
	default:
		return nil, "", fmt.Errorf("unsupported workload type for compute-agent grpc: %s", w.Type)
	}

	return req, id, nil
}

func resolveCompose(w models.Workload) (raw string, encoded string, err error) {
	if w.LocalPath != "" {
		b, readErr := os.ReadFile(w.LocalPath)
		if readErr != nil {
			return "", "", fmt.Errorf("failed to read local compose file %q: %w", w.LocalPath, readErr)
		}
		raw = string(b)
		encoded = base64.StdEncoding.EncodeToString(b)
		return raw, encoded, nil
	}
	if w.Compose == "" {
		return "", "", fmt.Errorf("compose content is required (set compose or local-path)")
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(w.Compose)
	if decodeErr == nil {
		return string(decoded), w.Compose, nil
	}
	return w.Compose, base64.StdEncoding.EncodeToString([]byte(w.Compose)), nil
}

func parseCommand(cmd string) []string {
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	return strings.Fields(cmd)
}

func parseControlVolumes(in []string) []*controlv1.VolumeMount {
	out := make([]*controlv1.VolumeMount, 0, len(in))
	for _, v := range in {
		parts := strings.Split(v, ":")
		if len(parts) < 2 {
			continue
		}
		vm := &controlv1.VolumeMount{HostPath: parts[0], ContainerPath: parts[1]}
		if len(parts) >= 3 {
			vm.ReadOnly = parts[2] == "ro"
		}
		out = append(out, vm)
	}
	return out
}

func parseControlPorts(in []string) []*controlv1.Port {
	out := make([]*controlv1.Port, 0, len(in))
	for _, p := range in {
		host, container, proto := parsePort(p)
		if host == 0 || container == 0 {
			continue
		}
		out = append(out, &controlv1.Port{HostPort: host, ContainerPort: container, Protocol: proto})
	}
	return out
}

func parseAgentVolumes(in []string) []*agentv1.VolumeMount {
	out := make([]*agentv1.VolumeMount, 0, len(in))
	for _, v := range in {
		parts := strings.Split(v, ":")
		if len(parts) < 2 {
			continue
		}
		vm := &agentv1.VolumeMount{HostPath: parts[0], ContainerPath: parts[1]}
		if len(parts) >= 3 {
			vm.ReadOnly = parts[2] == "ro"
		}
		out = append(out, vm)
	}
	return out
}

func parseAgentPorts(in []string) []*agentv1.PortMapping {
	out := make([]*agentv1.PortMapping, 0, len(in))
	for _, p := range in {
		host, container, proto := parsePort(p)
		if host == 0 || container == 0 {
			continue
		}
		out = append(out, &agentv1.PortMapping{HostPort: host, ContainerPort: container, Protocol: proto})
	}
	return out
}

func parsePort(in string) (host int32, container int32, proto string) {
	proto = "tcp"
	parts := strings.SplitN(in, "/", 2)
	if len(parts) == 2 && parts[1] != "" {
		proto = strings.ToLower(parts[1])
	}
	mapping := strings.Split(parts[0], ":")
	if len(mapping) != 2 {
		return 0, 0, proto
	}
	hostInt, err := strconv.Atoi(mapping[0])
	if err != nil {
		return 0, 0, proto
	}
	containerInt, err := strconv.Atoi(mapping[1])
	if err != nil {
		return 0, 0, proto
	}
	return int32(hostInt), int32(containerInt), proto
}

func workloadTypeToString(t agentv1.WorkloadType) string {
	switch t {
	case agentv1.WorkloadType_WORKLOAD_TYPE_CONTAINER:
		return "container"
	case agentv1.WorkloadType_WORKLOAD_TYPE_COMPOSE:
		return "compose"
	case agentv1.WorkloadType_WORKLOAD_TYPE_VM:
		return "vm"
	default:
		return "unknown"
	}
}
