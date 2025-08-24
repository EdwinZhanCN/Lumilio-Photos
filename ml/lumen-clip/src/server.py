"""
gRPC Server Runner for the Unified ML Service

This script initializes and runs the powerful UnifiedMLService, which handles
all CLIP and BioCLIP capabilities with built-in request batching for maximum
efficiency.
"""
import argparse
import logging
import signal
import sys
from concurrent import futures

import grpc

# Import the service definition and the unified service class
from proto import ml_service_pb2_grpc as rpc
from unified_service import UnifiedMLService

# --- Logging Configuration ---
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - [%(name)s] - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


def serve(port: int) -> None:
    """
    Initializes and starts the gRPC server for the UnifiedMLService.
    """
    # 1. Create the gRPC server
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    # 2. Instantiate and initialize the unified service
    service_instance = UnifiedMLService()
    rpc.add_InferenceServicer_to_server(service_instance, server)

    try:
        logger.info("Initializing unified models... This may take a moment.")
        service_instance.initialize()
    except Exception as e:
        logger.exception(f"Fatal error during model initialization: {e}")
        sys.exit(1)

    # 3. Start listening
    listen_addr = f"[::]:{port}"
    server.add_insecure_port(listen_addr)
    server.start()
    logger.info(f"✅ Unified ML Service is running and listening on {listen_addr}")
    logger.info("Supported tasks: clip_classify, bioclip_classify, smart_classify, clip_embed, bioclip_embed")

    # 4. Set up graceful shutdown
    def handle_shutdown(signum, frame):
        logger.info("Shutdown signal received. Stopping server...")
        server.stop(grace=5.0)

    signal.signal(signal.SIGINT, handle_shutdown)
    signal.signal(signal.SIGTERM, handle_shutdown)

    # 5. Wait for termination
    server.wait_for_termination()
    logger.info("Server shutdown complete.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Run the Unified gRPC ML Inference Server.")
    parser.add_argument(
        "--port",
        type=int,
        default=50051,
        help="The port number for the server to listen on.",
    )
    args = parser.parse_args()

    serve(port=args.port)
