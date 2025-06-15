# grpc API Definitions

## Service

```proto
service ImageClassificationService {
    rpc ClassifyImage(ClassifyImageRequest) returns (ClassifyImageResponse);
}
```

This service provides an endpoint for classifying images using Python Machine Learning Service.

## Messages

```proto
message ClassifyImageRequest {
    string image_url = 1;
    string model = 2;
}
```