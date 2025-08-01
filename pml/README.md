## pml stands for "Python Machine Learning"
This is the Machine Learning (ML) Service for Lumilio Photos
### Services
- **face_recognition[pending/not implemented]**: This service is responsible for recognizing faces in images.
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
uv pip install --index-url https://download.pytorch.org/whl/cu126 '.[gpu]'
```

**For CPU-only:**
```bash
uv pip install '.[cpu]'
```

### Docker

To be compatible with different platform, we use different Dockerfile for different of them, there are three files.

- [Dockerfile](./Dockerfile), the Dockerfile for CPU and macOS (mps/Metal Performance Shaders).
- [Dockerfile.cuda](./Dockerfile.cuda), the Dockerfile for CUDA GPUs (Nvidia), this project now use `CUDA12.6`, more information refers to [Nvidia Documentation](https://docs.nvidia.com/deeplearning/cudnn/backend/latest/reference/support-matrix.html)
- [Dockerfile.rocm](./Dockerfile.rocm), the Dockerfile for ROCm GPUs (AMD Instinct, Radeon PRO, Radeon RX 6600XT+, Ryzen AI Max 300 “Strix Halo”, etc.), this project now use `ROCm6.3`, more information refer to [AMD documentation](https://rocm.docs.amd.com/en/latest/compatibility/compatibility-matrix.html)

### Models Author Information
- **open-clip-torch**: This project uses the `open-clip-torch` library for image classification.
- **bioclip2**: This project uses the `bioclip2` library for biological_atlas.
```
@article{gu2025bioclip,
      title = {{B}io{CLIP} 2: Emergent Properties from Scaling Hierarchical Contrastive Learning}, 
      author = {Jianyang Gu and Samuel Stevens and Elizabeth G Campolongo and Matthew J Thompson and Net Zhang and Jiaman Wu and Andrei Kopanev and Zheda Mai and Alexander E. White and James Balhoff and Wasila M Dahdul and Daniel Rubenstein and Hilmar Lapp and Tanya Berger-Wolf and Wei-Lun Chao and Yu Su},
      year = {2025},
      eprint={2505.23883},
      archivePrefix={arXiv},
      primaryClass={cs.CV},
      url={https://arxiv.org/abs/2505.23883},
}
```






### License
This Project is licensed under the GPL-3.0 license. See the [LICENSE](../LICENSE) file for details.
