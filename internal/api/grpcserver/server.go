package grpcserver

import (
	"context"
	"encoding/json"
	"net"

	"github.com/jinziqi/execraft/internal/api/grpcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/jinziqi/execraft/internal/domain"
	"github.com/jinziqi/execraft/internal/engine"
	"github.com/jinziqi/execraft/internal/observability"
	"github.com/jinziqi/execraft/internal/store"
)

type Server struct {
	grpcpb.UnimplementedTaskServiceServer
	taskStore store.TaskStore
	sched     *engine.Scheduler
	metrics   *observability.Metrics
}

func New(taskStore store.TaskStore, sched *engine.Scheduler, metrics *observability.Metrics) *Server {
	return &Server{
		taskStore: taskStore,
		sched:     sched,
		metrics:   metrics,
	}
}

func Start(addr string, server *Server) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	g := grpc.NewServer()
	grpcpb.RegisterTaskServiceServer(g, server)
	go func() {
		_ = g.Serve(lis)
	}()
	return g, lis, nil
}

func (s *Server) SubmitTaskGraph(_ context.Context, req *structpb.Struct) (*structpb.Struct, error) {
	var graph domain.TaskGraph
	raw, err := json.Marshal(req.AsMap())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return nil, err
	}
	runID, taskIDs, err := s.sched.Submit(graph)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return mapToStruct(map[string]any{
		"run_id":   runID,
		"task_ids": taskIDs,
		"accepted": len(taskIDs),
	})
}

func (s *Server) GetTask(_ context.Context, req *structpb.Struct) (*structpb.Struct, error) {
	payload := req.AsMap()
	id, _ := payload["id"].(string)
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	task, ok, err := s.taskStore.Get(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "task not found")
	}
	return objectToStruct(task)
}

func (s *Server) ListTasks(_ context.Context, req *structpb.Struct) (*structpb.Struct, error) {
	payload := req.AsMap()
	statusValue, _ := payload["status"].(string)
	kindValue, _ := payload["kind"].(string)
	items, err := s.taskStore.List(store.TaskFilter{
		Status: domain.TaskStatus(statusValue),
		Kind:   kindValue,
	})
	if err != nil {
		return nil, err
	}
	return mapToStruct(map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	return mapToStruct(map[string]any{"status": "ok"})
}

func (s *Server) Metrics(_ context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	snapshot := s.metrics.Snapshot()
	out := make(map[string]any, len(snapshot))
	for k, v := range snapshot {
		out[k] = v
	}
	return mapToStruct(out)
}

func objectToStruct(v any) (*structpb.Struct, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}

func mapToStruct(m map[string]any) (*structpb.Struct, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}
	return structpb.NewStruct(normalized)
}
