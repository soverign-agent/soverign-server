package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	agentv1 "sovereign-ai-compliance/shared/proto/agent/v1"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:9088", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(fmt.Sprintf("dial: %v", err))
	}
	defer conn.Close()

	client := agentv1.NewAgentServiceClient(conn)
	resp, err := client.Invoke(ctx, &agentv1.AgentRequest{
		RequestId: "trigger-001",
		AgentId:   "supervisor",
		TenantId:  "tenant-demo",
		Payload:   []byte(`{"messages":[{"role":"user","content":"Explain EU AI Act transparency requirements"}]}`),
	})
	if err != nil {
		panic(fmt.Sprintf("invoke: %v", err))
	}

	fmt.Printf("Status: %v\n", resp.Status)
	fmt.Printf("Content: %s\n", string(resp.Result))
	fmt.Printf("Metadata: %+v\n", resp.Metadata)
	if resp.ErrorMessage != "" {
		fmt.Printf("Error: %s\n", resp.ErrorMessage)
	}
}
