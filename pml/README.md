## pml stands for "Python Machine Learning"
This is the Machine Learning (ML) Service for Lumilio Photos
### Services
- **face_recognition**: This service is responsible for recognizing faces in images. 
It uses a pre-trained model to identify and match faces in the photos.
- **image_classification**: This service is responsible for classifying images into different categories.

### Installation

This project uses `uv` for dependency management. You can install the required dependencies using the following commands, depending on your hardware:

**For Apple Silicon (OSX):**
```bash
uv pip install '.[osx]'
```

**For NVIDIA GPU (with CUDA):**
```bash
uv pip install --index-url https://download.pytorch.org/whl/cu121 '.[gpu]'
```

**For CPU-only:**
```bash
uv pip install '.[cpu]'
```

### Models Author Information
- **open-clip-torch**: This project uses the `open-clip-torch` library for image classification.

### License
This Project is licensed under the GPL-3.0 license. See the [LICENSE](../LICENSE) file for details.