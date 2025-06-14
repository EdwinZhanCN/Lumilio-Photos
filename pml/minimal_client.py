#!/usr/bin/env python3
"""
Minimal client for CLIP gRPC service

This client demonstrates basic usage of the ML prediction server by:
1. Checking service health
2. Processing the dog.jpeg image with CLIP
3. Getting text embeddings
"""

import grpc
import sys
import os
import time

# Add the current directory to path for imports
sys.path.append(os.path.dirname(__file__))

from proto import ml_service_pb2, ml_service_pb2_grpc


def main():
    """Main function to test CLIP service with dog.jpeg"""

    # Server configuration
    server_address = 'localhost:50051'
    image_path = 'dog.jpeg'

    print("=" * 60)
    print("Minimal CLIP gRPC Client")
    print("=" * 60)

    # Check if dog.jpeg exists
    if not os.path.exists(image_path):
        print(f"❌ Error: {image_path} not found in current directory")
        print("Please make sure dog.jpeg is in the same directory as this script")
        return 1

    # Connect to server
    print(f"🔗 Connecting to server at {server_address}...")
    try:
        channel = grpc.insecure_channel(server_address)
        stub = ml_service_pb2_grpc.PredictionServiceStub(channel)
        print("✅ Connected successfully")
    except Exception as e:
        print(f"❌ Failed to connect: {e}")
        print("Make sure the server is running with: python server.py")
        return 1

    try:
        # 1. Health Check
        print("\n📊 Checking service health...")
        health_request = ml_service_pb2.HealthCheckRequest(service_name="clip")
        try:
            health_response = stub.HealthCheck(health_request)
            status_names = {0: "UNKNOWN", 1: "SERVING", 2: "NOT_SERVING", 3: "ERROR"}
            status_name = status_names.get(health_response.status, "UNKNOWN")

            print(f"   Status: {status_name}")
            print(f"   Model: {health_response.model_name} v{health_response.model_version}")
            print(f"   Uptime: {health_response.uptime_seconds} seconds")
            print(f"   Message: {health_response.message}")

            if health_response.status != 1:  # Not SERVING
                print("❌ Service is not healthy. Cannot proceed with tests.")
                return 1

        except grpc.RpcError as e:
            print(f"❌ Health check failed: {e}")
            return 1

        # 2. Process dog.jpeg with CLIP
        print(f"\n🖼️  Processing {image_path} with CLIP...")

        # Read image file
        with open(image_path, 'rb') as f:
            image_data = f.read()

        # Test with some common labels
        target_labels = ["dog", "cat", "bird", "car", "house"]

        image_request = ml_service_pb2.ImageProcessRequest(
            image_id="dog_test_001",
            image_data=image_data,
            target_labels=target_labels
        )

        try:
            start_time = time.time()
            image_response = stub.ProcessImageForCLIP(image_request)
            elapsed_time = time.time() - start_time

            print(f"✅ Image processed successfully!")
            print(f"   Image ID: {image_response.image_id}")
            print(f"   Model Version: {image_response.model_version}")
            print(f"   Processing Time: {image_response.processing_time_ms}ms")
            print(f"   Total Time: {elapsed_time*1000:.1f}ms")
            print(f"   Feature Vector Size: {len(image_response.image_feature_vector)}")
            print(f"   Similarity Score: {image_response.similarity_score:.4f}")

            if image_response.predicted_labels:
                print(f"   Predictions:")
                for i, label in enumerate(image_response.predicted_labels[:3], 1):
                    print(f"     {i}. {label}")

        except grpc.RpcError as e:
            print(f"❌ Image processing failed: {e}")
            return 1

        # 3. Test text embedding
        print(f"\n📝 Getting text embedding...")

        text_request = ml_service_pb2.TextEmbeddingRequest(
            text="a photo of a dog"
        )

        try:
            text_response = stub.GetTextEmbeddingForCLIP(text_request)

            print(f"✅ Text embedding generated successfully!")
            print(f"   Text: 'a photo of a dog'")
            print(f"   Model Version: {text_response.model_version}")
            print(f"   Processing Time: {text_response.processing_time_ms}ms")
            print(f"   Feature Vector Size: {len(text_response.text_feature_vector)}")

        except grpc.RpcError as e:
            print(f"❌ Text embedding failed: {e}")
            return 1

        print("\n" + "=" * 60)
        print("🎉 All tests completed successfully!")
        print("The CLIP service is working correctly.")
        print("=" * 60)

        return 0

    finally:
        channel.close()


if __name__ == "__main__":
    exit_code = main()
    sys.exit(exit_code)
