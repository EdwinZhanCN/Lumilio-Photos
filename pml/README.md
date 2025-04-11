## pml stands for "Python Machine Learning"
This is the Machine Learning (ML) Service for Lumilio Photos
### Services
- **face_recognition**: This service is responsible for recognizing faces in images. 
It uses a pre-trained model to identify and match faces in the photos.
- **image_classification**: This service is responsible for classifying images into different categories.

### Models Author Information
- **mobileclip_s1.pt**: This is a pre-trained model for image classification. This model was obtained from [apple/ml-mobileclip](https://github.com/apple/ml-mobileclip)
- **yolov8-lite-s.pt** This is a pre-trained model for face classification and recognition. This model was obtained from [derronqi/yolov8-face](https://github.com/derronqi/yolov8-face)

### License
- The model `mobileclip_s1.pt` is licensed under the [MIT](https://github.com/apple/ml-mobileclip/blob/main/LICENSE) license. 
- The model `yolov8-lite-s.pt` is licensed under the [GPL-3.0](https://github.com/derronqi/yolov8-face/blob/main/LICENSE) license.

This Project is licensed under the GPL-3.0 license. See the [LICENSE](../LICENSE) file for details.