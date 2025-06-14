FROM python:3.10-slim

WORKDIR /app

COPY pml/requirements.txt ./
RUN apt-get update && \
    apt-get install -y git && \
    apt-get install -y wget && \
    pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt python-dotenv

COPY . .

# 创建模型目录
RUN mkdir -p /app/models

# 下载模型（示例：用wget，替换为你的实际下载链接）
RUN wget -O /app/models/mobileclip_s1.pt https://docs-assets.developer.apple.com/ml-research/datasets/mobileclip/mobileclip_s1.pt \
    || (echo "Apple Download Link Down, Please Create an Issue at https://github.com/EdwinZhanCN/Lumilio-Photos" && exit 1)

# 如有其他模型，也可类似下载
# RUN wget -O /app/models/yolov8-lite-s.pt https://your-model-download-link/yolov8-lite-s.pt

ENV PYTHONUNBUFFERED=1
ENV MODEL_PATH=/app/models

EXPOSE 8081

CMD ["python", "pml/server.py", "--port", "8081", "--workers", "10"]
