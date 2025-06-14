import torch
from PIL import Image
import mobileclip
import json  # 新增json模块导入

model, _, preprocess = mobileclip.create_model_and_transforms('mobileclip_s1', pretrained='./pt/mobileclip_s1.pt')
tokenizer = mobileclip.get_tokenizer('mobileclip_s1')

image = preprocess(Image.open("dog.jpeg").convert('RGB')).unsqueeze(0)
# text = tokenizer(["a diagram", "a dog", "a cat"])

# 加载ImageNet类别映射（适配标准格式）
with open('./imagenet_class_index.json') as f:
    imagenet_class_dict = json.load(f)  # 标准格式为 {"0": ["n01440764", "tench"], ...}
    text_descriptions = [
        f"a photo of a {class_info[1]}" 
        for class_info in imagenet_class_dict.values()
    ]
    imagenet_labels = {
        int(idx): {"id": class_info[0], "en": class_info[1]} 
        for idx, class_info in imagenet_class_dict.items()
    }

# 分批处理文本特征（避免内存溢出）
batch_size = 100
all_text_features = []

with torch.no_grad(), torch.cuda.amp.autocast():
    image_features = model.encode_image(image)
    
    # 分批处理文本
    for i in range(0, len(text_descriptions), batch_size):
        batch_text = tokenizer(text_descriptions[i:i+batch_size])
        text_features = model.encode_text(batch_text)
        text_features /= text_features.norm(dim=-1, keepdim=True)
        all_text_features.append(text_features)
    
    # 合并所有文本特征
    text_features = torch.cat(all_text_features, dim=0)
    
    # 计算相似度（保持原有逻辑）
    text_probs = (100.0 * image_features @ text_features.T).softmax(dim=-1)
    top5_probs, top5_indices = torch.topk(text_probs, 5, dim=-1)

# 结果处理
top5_probs = top5_probs.cpu().numpy()[0]
top5_indices = top5_indices.cpu().numpy()[0]

print("\nTop5 预测结果：")
for rank, (idx, prob) in enumerate(zip(top5_indices, top5_probs), 1):
    label = imagenet_labels[idx]
    print(f"{rank}. [{label['id']}] {label['en']}: {prob*100:.2f}%")