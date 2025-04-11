import os
import time
import grpc
from concurrent import futures
import torch
import numpy as np
from datetime import datetime

# 导入生成的 gRPC 文件
import prediction_pb2
import prediction_pb2_grpc

# 加载 PyTorch 模型
class ModelService:
    def __init__(self, model_path="model.pth"):
        self.model_path = model_path
        self.model_version = os.environ.get("MODEL_VERSION", "1.0.0")
        self.start_time = time.time()
        self.load_model()

    def load_model(self):
        try:
            # 这里替换为您的实际模型加载逻辑
            self.model = torch.load(self.model_path) if os.path.exists(self.model_path) else self._get_dummy_model()
            self.model.eval()  # 设置为评估模式
            print(f"Model loaded successfully, version: {self.model_version}")
        except Exception as e:
            print(f"Error loading model: {e}")
            # 加载失败时使用简单模型作为后备
            self.model = self._get_dummy_model()

    def _get_dummy_model(self):
        # 创建一个简单的线性模型作为示例
        model = torch.nn.Sequential(
            torch.nn.Linear(10, 64),
            torch.nn.ReLU(),
            torch.nn.Linear(64, 32),
            torch.nn.ReLU(),
            torch.nn.Linear(32, 5)  # 输出5个值的数组
        )
        model.eval()
        return model

    def predict(self, features):
        """执行预测并返回结果"""
        try:
            with torch.no_grad():
                # 将输入转换为 PyTorch 张量
                input_tensor = torch.tensor(features, dtype=torch.float32).unsqueeze(0)
                # 执行预测
                output = self.model(input_tensor)
                # 计算置信度 (示例: 使用softmax后的最大值)
                probabilities = torch.nn.functional.softmax(output, dim=1)
                confidence = torch.max(probabilities).item()
                # 返回预测结果和置信度
                return output.squeeze(0).tolist(), confidence
        except Exception as e:
            print(f"Prediction error: {e}")
            # 出错时返回零数组和零置信度
            return [0.0] * 5, 0.0

    def get_uptime_seconds(self):
        """返回服务运行时间（秒）"""
        return int(time.time() - self.start_time)


# gRPC 服务实现
class PredictionServicer(prediction_pb2_grpc.PredictionServiceServicer):
    def __init__(self, model_service):
        self.model_service = model_service

    def Predict(self, request, context):
        """处理单个预测请求"""
        features = list(request.features)
        predictions, confidence = self.model_service.predict(features)

        return prediction_pb2.PredictResponse(
            prediction=predictions,
            confidence=confidence,
            model_version=self.model_service.model_version,
            prediction_time=int(time.time() * 1000)  # 毫秒时间戳
        )

    def BatchPredict(self, request, context):
        """处理批量预测请求"""
        responses = []
        success_count = 0

        for single_request in request.requests:
            try:
                features = list(single_request.features)
                predictions, confidence = self.model_service.predict(features)

                response = prediction_pb2.PredictResponse(
                    prediction=predictions,
                    confidence=confidence,
                    model_version=self.model_service.model_version,
                    prediction_time=int(time.time() * 1000)
                )
                responses.append(response)
                success_count += 1
            except Exception as e:
                print(f"Error in batch prediction: {e}")
                # 添加一个空响应
                responses.append(prediction_pb2.PredictResponse(
                    prediction=[0.0] * 5,
                    confidence=0.0,
                    model_version=self.model_service.model_version,
                    prediction_time=int(time.time() * 1000)
                ))

        return prediction_pb2.BatchPredictResponse(
            responses=responses,
            success_count=success_count,
            failed_count=len(request.requests) - success_count
        )

    def HealthCheck(self, request, context):
        """健康检查"""
        return prediction_pb2.HealthCheckResponse(
            status=prediction_pb2.HealthCheckResponse.ServingStatus.SERVING,
            model_version=self.model_service.model_version,
            uptime_seconds=self.model_service.get_uptime_seconds()
        )


def serve():
    # 配置服务器
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    # 创建模型服务
    model_service = ModelService(model_path=os.environ.get("MODEL_PATH", "model.pth"))

    # 注册 gRPC 服务
    prediction_pb2_grpc.add_PredictionServiceServicer_to_server(
        PredictionServicer(model_service), server
    )

    # 定义服务地址
    port = os.environ.get("GRPC_PORT", "50051")
    server.add_insecure_port(f"[::]:{port}")

    # 启动服务
    server.start()
    print(f"PyTorch prediction service started on port {port}")

    try:
        # 保持服务运行
        server.wait_for_termination()
    except KeyboardInterrupt:
        print("Shutting down server...")
        server.stop(0)


if __name__ == "__main__":
    serve()