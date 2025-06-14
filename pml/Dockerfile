FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt ./
RUN pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

COPY . .

# 创建模型目录
RUN mkdir -p pt

# 下载模型（示例：用wget，替换为你的实际下载链接）
RUN wget -O pt/mobileclip_s1.pt https://docs-assets.developer.apple.com/ml-research/datasets/mobileclip/mobileclip_s1.pt \
    || (echo "Apple Download Link Down, Please Create an Issue at https://github.com/EdwinZhanCN/Lumilio-Photos" && exit 1)

# 如有其他模型，也可类似下载
# RUN wget -O pt/yolov8-lite-s.pt https://your-model-download-link/yolov8-lite-s.pt

ENV PYTHONUNBUFFERED=1

EXPOSE 50051

CMD ["python", "pml/server.py", "--port", "50051", "--workers", "10"]
