# Contributing Photo Frames

Welcome to the RKPhoto Manager photo frames system! This guide will help you understand how to create and contribute new photo frames to the project.

## Overview

The photo frames system is designed to be simple and user-friendly. Each frame is a pre-designed style with fixed, beautiful settings - no complex customization options needed. This keeps the web experience fast and intuitive.

## Simplified Architecture

### Core Components

1. **SimpleFramePicker**: The main component that handles frame selection and rendering
2. **Frame Registry**: Central registry that manages available frame definitions
3. **Canvas Rendering**: Direct canvas drawing functions for each frame type

### File Structure

```
src/components/PhotoFrames/
├── types.ts                 # Type definitions
├── frameRegistry.ts         # Frame registry and definitions
├── SimpleFramePicker.tsx    # Main frame picker component
├── index.ts                 # Exports
├── CONTRIBUTING.md          # This file
└── README.md               # Documentation
```

## Adding a New Frame

### Step 1: Add Frame Definition

Add your frame to the `frameRegistry.ts` file:

```typescript
// In frameDefinitions array, add:
{
    id: "my-awesome-frame",
    name: "My Awesome Frame",
    description: "A fantastic frame that does amazing things with colors and shapes.",
    author: "Your Name",
    version: "1.0.0",
    tags: ["modern", "colorful", "creative"],
    supportsMetadata: true,
    component: null, // Not needed in simplified approach
},
```

### Step 2: Add Drawing Function

Add your drawing function to `SimpleFramePicker.tsx`:

```typescript
const drawMyAwesomeFrame = async (
  ctx: CanvasRenderingContext2D,
  img: HTMLImageElement,
) => {
  const borderWidth = 25; // Fixed, good-looking value
  const frameColor = "#FF6B6B"; // Fixed, attractive color
  
  // Set canvas size
  ctx.canvas.width = img.width + borderWidth * 2;
  ctx.canvas.height = img.height + borderWidth * 2;
  
  // Clear canvas
  ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  
  // Draw your frame
  ctx.fillStyle = frameColor;
  ctx.fillRect(0, 0, ctx.canvas.width, ctx.canvas.height);
  
  // Draw the image
  ctx.drawImage(img, borderWidth, borderWidth);
  
  // Add your special effects here
  // Examples: gradients, shadows, textures, etc.
};
```

### Step 3: Add to Switch Statement

Update the switch statement in `generateFramedImage()`:

```typescript
switch (selectedFrame.id) {
  case "glass-frame":
    await drawGlassFrame(ctx, img);
    break;
  case "classic-frame":
    await drawClassicFrame(ctx, img);
    break;
  case "polaroid-frame":
    await drawPolaroidFrame(ctx, img);
    break;
  case "my-awesome-frame": // Add this
    await drawMyAwesomeFrame(ctx, img);
    break;
  default:
    await drawSimpleFrame(ctx, img);
}
```

## Design Principles

### 1. Fixed, Beautiful Settings
- No sliders, color pickers, or complex options
- Choose values that look great for most photos
- Test with various image types and sizes

### 2. Performance First
- Keep drawing functions efficient
- Minimize complex calculations
- Use simple, fast canvas operations

### 3. Web-Appropriate
- Remember this runs in browsers, not desktop software
- Keep file sizes reasonable
- Ensure smooth user experience

## Frame Drawing Best Practices

### Canvas Setup
```typescript
// Always set canvas size first
ctx.canvas.width = desiredWidth;
ctx.canvas.height = desiredHeight;

// Clear the canvas
ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
```

### Adding Effects
```typescript
// Shadows
ctx.save();
ctx.shadowColor = "rgba(0, 0, 0, 0.3)";
ctx.shadowBlur = 20;
ctx.shadowOffsetX = 5;
ctx.shadowOffsetY = 5;
// Draw your shape
ctx.restore();

// Gradients
const gradient = ctx.createLinearGradient(x1, y1, x2, y2);
gradient.addColorStop(0, "color1");
gradient.addColorStop(1, "color2");
ctx.fillStyle = gradient;
```

### Metadata Support
If your frame supports metadata display:

```typescript
if (metadata) {
  ctx.fillStyle = "#333333";
  ctx.font = "14px Arial, sans-serif";
  ctx.textAlign = "center";
  ctx.fillText(metadata, x, y);
}
```

## Good Frame Ideas

### Style Categories
- **Modern**: Clean lines, minimal borders, subtle effects
- **Vintage**: Aged textures, warm colors, decorative elements
- **Artistic**: Creative shapes, interesting patterns, unique styles
- **Professional**: Clean, business-appropriate, elegant

### Technical Inspiration
- **Material Design**: Elevation, shadows, clean geometry
- **Polaroid**: White borders with signature bottom area
- **Film Strip**: Multiple exposure looks, sprocket holes
- **Gallery**: Museum-style matting and frames
- **Digital**: Glitch effects, pixel art, neon themes

## Testing Your Frame

### Manual Testing Checklist
- [ ] Test with landscape images
- [ ] Test with portrait images  
- [ ] Test with square images
- [ ] Test with very large images
- [ ] Test with very small images
- [ ] Test with and without metadata
- [ ] Verify export functionality works
- [ ] Check performance on slower devices

### Visual Quality
- Does the frame enhance the photo?
- Are the proportions balanced?
- Do colors work well with different photo types?
- Is the frame distinct but not overwhelming?

## Submission Guidelines

### Pull Request Checklist
- [ ] Frame definition added to registry
- [ ] Drawing function implemented
- [ ] Switch statement updated
- [ ] Frame tested with various images
- [ ] Code follows existing style
- [ ] No complex configuration options added
- [ ] Performance is acceptable

### Code Quality
- Use consistent naming conventions
- Add comments for complex drawing logic
- Handle edge cases gracefully
- Keep functions focused and readable

### Documentation
- Choose descriptive frame names
- Write clear descriptions
- Use appropriate tags for categorization
- Credit yourself as the author

## Examples

Look at the existing frame implementations in `SimpleFramePicker.tsx`:

- **`drawGlassFrame`**: Modern glassmorphism with gradients and shadows
- **`drawClassicFrame`**: Traditional wooden frame with layered borders
- **`drawPolaroidFrame`**: Instant camera style with vintage effects

These show different approaches to creating attractive, fixed-setting frames.

## Getting Help

If you need help:
1. Study the existing frame drawing functions
2. Review the Canvas API documentation
3. Open an issue on GitHub for technical questions
4. Ask in community discussions

## License

By contributing frames to this project, you agree to license your contributions under the same license as the project.

---

Thank you for contributing to RKPhoto Manager! Your creativity helps make this project better for everyone.