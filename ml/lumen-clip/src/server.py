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
import os
import socket
import uuid
from zeroconf import Zeroconf, ServiceInfo

# Import the service definition and the unified service class
from proto import ml_service_pb2_grpc as rpc
from service_registry import UnifiedMLService

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
    # Advertise service over mDNS using zeroconf
    zeroconf = None
    mdns_info = None
    try:
        if Zeroconf is not None and ServiceInfo is not None:
            # Determine advertised IP:
            # 1) ADVERTISE_IP env overrides
            # 2) Best-effort LAN IP via UDP trick
            # 3) Fallback to hostname resolution
            ip = os.getenv("ADVERTISE_IP")
            if not ip:
                try:
                    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
                    s.connect(("8.8.8.8", 80))
                    ip = s.getsockname()[0]
                    s.close()
                except Exception:
                    ip = socket.gethostbyname(socket.gethostname())
            if ip.startswith("127."):
                logger.warning("mDNS is advertising loopback IP %s; other devices may not reach the service. Set ADVERTISE_IP to a LAN IP.", ip)
            # TXT record fields: uuid/status/version can be overridden via env
            props = {
                "uuid": os.getenv("CLIP_MDNS_UUID", str(uuid.uuid4())),
                "status": os.getenv("CLIP_MDNS_STATUS", "ready"),
                "version": os.getenv("CLIP_MDNS_VERSION", "1.0.0"),
            }
            mdns_info = ServiceInfo(
                type_="_homenative-node._tcp.local.",
                name="CLIP-Image-Proccesor._homenative-node._tcp.local.",
                addresses=[socket.inet_aton(ip)],
                port=port,
                properties=props,
                server=f"{socket.gethostname()}.local.",
            )
            zeroconf = Zeroconf()
            zeroconf.register_service(mdns_info)
            logger.info("mDNS advertised: CLIP-Image-Proccesor._homenative-node._tcp.local. at %s:%d", ip, port)
        else:
            logger.warning("Zeroconf not installed; skipping mDNS advertisement.")
    except Exception as e:
        logger.warning("mDNS advertisement failed: %s", e)
        zeroconf = None
        mdns_info = None
    logger.info("Supported tasks: clip_classify, bioclip_classify, smart_classify, clip_embed, bioclip_embed, clip_image_embed, bioclip_image_embed")

    # 4. Set up graceful shutdown
    def handle_shutdown(signum, frame):
        logger.info("Shutdown signal received. Stopping server...")
        try:
            if zeroconf and mdns_info:
                zeroconf.unregister_service(mdns_info)
                zeroconf.close()
        except Exception as e:
            logger.warning("mDNS unregistration failed: %s", e)
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
