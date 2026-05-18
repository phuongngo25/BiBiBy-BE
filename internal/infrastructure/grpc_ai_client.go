package infrastructure

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"nutrix-backend/internal/domain"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/inferencev1" // Đường dẫn import code gen
)

type grpcAIClient struct {
	client pb.InferenceServiceClient
	conn   *grpc.ClientConn
}

// NewGrpcAIClient khởi tạo kết nối gRPC tới AI server (VD: localhost:50051).
func NewGrpcAIClient(targetURI string) (domain.InferencePort, error) {
	// Sử dụng DialContext với timeout 5s cho việc kết nối ban đầu
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Trong môi trường Production, bạn nên cấu hình TLS/SSL thay vì insecure
	conn, err := grpc.DialContext(ctx, targetURI,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // Đợi kết nối thành công trước khi trả về
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI microservice at %s: %w", targetURI, err)
	}

	return &grpcAIClient{
		client: pb.NewInferenceServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close giải phóng tài nguyên connection.
func (g *grpcAIClient) Close() error {
	return g.conn.Close()
}

func (g *grpcAIClient) EstimateVolume(ctx context.Context, imageBytes []byte) (float64, error) {
	// Set Deadline cứng 30s cho request này như AI team yêu cầu
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.EstimateVolumeRequest{
		ImageData: imageBytes,
	}

	start := time.Now()
	res, err := g.client.EstimateVolume(reqCtx, req)
	if err != nil {
		log.Printf("[gRPC] EstimateVolume failed after %v: %v", time.Since(start), err)
		return 0, fmt.Errorf("ai inference failed: %w", err)
	}

	log.Printf("[gRPC] AI Success | ReqID: %s | Latency: %.2fms | Volume: %.2f cm³ | Conf: %.2f",
		res.RequestId, res.LatencyMs, res.VolumeCm3, res.Confidence)

	// Có thể throw lỗi nội bộ nếu Confidence quá thấp (< 0.5)
	if res.Confidence < 0.5 {
		return 0, fmt.Errorf("ai confidence too low: %.2f", res.Confidence)
	}

	return float64(res.VolumeCm3), nil
}
