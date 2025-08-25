#!/usr/bin/env python
import argparse
import os
import sys
from typing import List, Tuple, Any
import torch

# Add the project root to path so we can import the module
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.abspath(os.path.join(script_dir, "../../.."))
sys.path.append(project_root)

from src.biological_atlas.bioclip_model import BioCLIPModelManager


def extract_species_name(label_data: Any) -> str:
    """
    Extract the species name from the complex label structure.

    Args:
        label_data: The label data from the model

    Returns:
        The species name as a string
    """
    if isinstance(label_data, list) and len(label_data) == 2 and isinstance(label_data[1], str):
        # Format: [['Animalia', ...], 'Wompoo Fruit Dove']
        return label_data[1]
    elif isinstance(label_data, str):
        # It's already a string
        return label_data
    else:
        # Try to convert to string as fallback
        return str(label_data)


def classify_image(image_path: str, top_k: int = 3, use_raw: bool = True) -> List[Tuple[str, float]]:
    """
    Classify an image using BioCLIP model and return top_k predictions.

    Args:
        image_path: Path to the image file
        top_k: Number of top predictions to return
        use_raw: Use raw similarity scores instead of softmax probabilities

    Returns:
        List of (species_name, similarity/probability) tuples
    """
    # Check if image file exists
    if not os.path.exists(image_path):
        raise FileNotFoundError(f"Image file not found: {image_path}")

    # Initialize the model
    print("Initializing BioCLIP model...")
    model = BioCLIPModelManager()
    model.initialize()

    print(f"Model initialized, classifying image: {image_path}")

    # Read the image file
    with open(image_path, "rb") as f:
        image_bytes = f.read()

    # Classify the image
    model._ensure_initialized()
    img_vec = torch.tensor(model.encode_image(image_bytes), device=model.device).unsqueeze(0)
    assert model.text_embeddings is not None
    text_emb = model.text_embeddings.to(model.device)

    with torch.no_grad():
        # Calculate raw similarity scores
        raw_sims = img_vec @ text_emb.T

        if use_raw:
            # Use raw similarity scores (cosine similarity)
            raw_probs, raw_idxs = raw_sims.squeeze(0).topk(min(top_k, raw_sims.numel()))
            results = []
            for i, idx in enumerate(raw_idxs):
                species_name = extract_species_name(model.labels[idx])
                # Normalize raw scores to 0-1 range for easier interpretation
                # Cosine similarity is in [-1,1], so we rescale to [0,1]
                similarity = (raw_probs[i].item() + 1) / 2
                results.append((species_name, similarity))
            return results
        else:
            # Use softmax probabilities (original approach)
            sims = raw_sims.softmax(dim=-1).squeeze(0)
            probs, idxs = sims.topk(min(top_k, sims.numel()))

            # Extract just the species name and return with the probability
            results = []
            for i, idx in enumerate(idxs):
                species_name = extract_species_name(model.labels[idx])
                probability = probs[i].item()
                results.append((species_name, probability))

            return results


def main():
    """Main entry point for the script."""
    # Set up argument parser
    parser = argparse.ArgumentParser(description="Classify an image using BioCLIP model")
    parser.add_argument("image_path", help="Path to the image file to classify")
    parser.add_argument("--top-k", type=int, default=3,
                        help="Number of top predictions to return (default: 3)")
    parser.add_argument("--use-softmax", action="store_true",
                        help="Use softmax probabilities instead of raw similarity scores")

    args = parser.parse_args()

    try:
        # Classify the image
        predictions = classify_image(
            args.image_path,
            args.top_k,
            use_raw=not args.use_softmax
        )

        # Print results
        print("\nClassification results:")
        print("-" * 50)

        for i, (species, score) in enumerate(predictions, 1):
            if args.use_softmax and score < 0.0001:
                # Use scientific notation for very small probabilities
                print(f"{i}. {species}: {score:.10e} ({score*100:.6f}%)")
            else:
                # Use regular formatting for similarity scores or larger probabilities
                print(f"{i}. {species}: {score:.4f} ({score*100:.2f}%)")
        print("-" * 50)

        if not args.use_softmax:
            print("\nNote: Using raw similarity scores (0-1 scale) instead of probabilities.")
            print("Higher values indicate better matches.")

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
