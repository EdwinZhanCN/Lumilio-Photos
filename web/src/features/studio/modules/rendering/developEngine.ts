/**
 * The persistent develop pipeline: the fix at the heart of the rewrite.
 *
 * WORKER-SAFE. No DOM.
 *
 * The old worker rebuilt everything every frame — new texture, freshly compiled
 * shader, new pipeline, a full GPU→CPU readback — so dragging a slider paid for
 * a cold start each time. This engine does the expensive work once in `init`
 * (upload the source texture with mipmaps, compile and link the program, bind
 * the quad) and then `render` only updates uniforms and draws. It renders
 * straight onto its own OffscreenCanvas, which the compose stage reads with
 * `drawImage` — the pixels never leave the GPU.
 *
 * WebGL2 only for now; a WebGPU backend behind the same shape is Phase 1.5.
 * Geometry (crop/rotate/flip) and composition are deliberately NOT here — this
 * stage is pure per-pixel color, so it works on one stable full-frame texture.
 */

import type { StudioEditAdjustments } from "../../model/editTypes";
import { clampToTexture } from "./coordinateSystem";
import { WEBGL_FRAGMENT_SHADER, WEBGL_VERTEX_SHADER } from "./developShaders";

const UNIFORM_NAMES = [
  "u_exposure",
  "u_contrast",
  "u_highlights",
  "u_shadows",
  "u_whites",
  "u_blacks",
  "u_temperature",
  "u_tint",
  "u_vibrance",
  "u_saturation",
  "u_clarity",
  "u_sharpness",
  "u_noiseReduction",
] as const;

type UniformName = (typeof UNIFORM_NAMES)[number];

const ADJUSTMENT_FOR_UNIFORM: Record<UniformName, keyof StudioEditAdjustments> = {
  u_exposure: "exposure",
  u_contrast: "contrast",
  u_highlights: "highlights",
  u_shadows: "shadows",
  u_whites: "whites",
  u_blacks: "blacks",
  u_temperature: "temperature",
  u_tint: "tint",
  u_vibrance: "vibrance",
  u_saturation: "saturation",
  u_clarity: "clarity",
  u_sharpness: "sharpness",
  u_noiseReduction: "noiseReduction",
};

function compileShader(gl: WebGL2RenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error("Failed to create WebGL shader");
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const info = gl.getShaderInfoLog(shader) ?? "Unknown WebGL shader error";
    gl.deleteShader(shader);
    throw new Error(info);
  }
  return shader;
}

export class DevelopEngine {
  private readonly canvas: OffscreenCanvas;
  private readonly gl: WebGL2RenderingContext;
  private readonly program: WebGLProgram;
  private readonly texture: WebGLTexture;
  private readonly uniforms: Record<UniformName, WebGLUniformLocation | null>;
  private readonly uTextureSize: WebGLUniformLocation | null;

  /**
   * Effective source size = the uploaded texture size, after the GPU guardrail.
   * This is the ceiling an export can reach; {@link originalSourceWidth} keeps
   * the true source size for reporting a guardrail downscale.
   */
  readonly sourceWidth: number;
  readonly sourceHeight: number;
  readonly originalSourceWidth: number;
  readonly originalSourceHeight: number;
  /** GPU limit callers clamp export size against (Phase 2 guardrail). */
  readonly maxTextureSize: number;

  private constructor(source: ImageBitmap) {
    this.originalSourceWidth = source.width;
    this.originalSourceHeight = source.height;

    this.canvas = new OffscreenCanvas(1, 1);
    const gl = this.canvas.getContext("webgl2", {
      premultipliedAlpha: false,
      preserveDrawingBuffer: true,
      antialias: false,
    });
    if (!gl) throw new Error("WebGL2 is unavailable");
    this.gl = gl;
    this.maxTextureSize = gl.getParameter(gl.MAX_TEXTURE_SIZE) as number;

    // The guardrail: a source larger than the GPU can hold is downscaled once,
    // here, to the largest same-aspect texture it can upload.
    const fit = clampToTexture(source.width, source.height, this.maxTextureSize);
    this.sourceWidth = fit.width;
    this.sourceHeight = fit.height;

    // --- Program (compiled once) ---
    const vertexShader = compileShader(gl, gl.VERTEX_SHADER, WEBGL_VERTEX_SHADER);
    const fragmentShader = compileShader(gl, gl.FRAGMENT_SHADER, WEBGL_FRAGMENT_SHADER);
    const program = gl.createProgram();
    if (!program) throw new Error("Failed to create WebGL program");
    gl.attachShader(program, vertexShader);
    gl.attachShader(program, fragmentShader);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      throw new Error(gl.getProgramInfoLog(program) ?? "Failed to link WebGL program");
    }
    gl.deleteShader(vertexShader);
    gl.deleteShader(fragmentShader);
    this.program = program;
    gl.useProgram(program);

    // --- Full-screen quad (bound once) ---
    const vao = gl.createVertexArray();
    const buffer = gl.createBuffer();
    if (!vao || !buffer) throw new Error("Failed to allocate WebGL geometry");
    gl.bindVertexArray(vao);
    // pos.xy, uv.xy — clip-space top (+y) maps to uv v=0 so the canvas presents
    // upright when read back with drawImage (no post-render flip needed).
    const vertices = new Float32Array([-1, -1, 0, 1, 1, -1, 1, 1, -1, 1, 0, 0, 1, 1, 1, 0]);
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);
    const positionLocation = gl.getAttribLocation(program, "a_position");
    const uvLocation = gl.getAttribLocation(program, "a_uv");
    gl.enableVertexAttribArray(positionLocation);
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 16, 0);
    gl.enableVertexAttribArray(uvLocation);
    gl.vertexAttribPointer(uvLocation, 2, gl.FLOAT, false, 16, 8);

    // --- Source texture (uploaded once, mipmapped for clean downscale) ---
    const texture = gl.createTexture();
    if (!texture) throw new Error("Failed to allocate WebGL texture");
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 0);
    let upload: TexImageSource = source;
    if (fit.clamped) {
      const scaled = new OffscreenCanvas(fit.width, fit.height);
      const scaledCtx = scaled.getContext("2d");
      if (!scaledCtx) throw new Error("2D context is unavailable for source downscale");
      scaledCtx.imageSmoothingEnabled = true;
      scaledCtx.imageSmoothingQuality = "high";
      scaledCtx.drawImage(source, 0, 0, fit.width, fit.height);
      upload = scaled;
    }
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, upload);
    gl.generateMipmap(gl.TEXTURE_2D);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    this.texture = texture;

    // --- Uniform locations (resolved once) ---
    gl.uniform1i(gl.getUniformLocation(program, "u_image"), 0);
    this.uTextureSize = gl.getUniformLocation(program, "u_textureSize");
    this.uniforms = UNIFORM_NAMES.reduce(
      (acc, name) => {
        acc[name] = gl.getUniformLocation(program, name);
        return acc;
      },
      {} as Record<UniformName, WebGLUniformLocation | null>,
    );
  }

  static create(source: ImageBitmap): DevelopEngine {
    return new DevelopEngine(source);
  }

  /**
   * Develop the source at `width`×`height` and return the engine's canvas.
   * Only uniforms change between calls; the texture and program are reused.
   */
  render(adjustments: StudioEditAdjustments, width: number, height: number): OffscreenCanvas {
    const gl = this.gl;
    const w = Math.max(1, Math.round(width));
    const h = Math.max(1, Math.round(height));
    if (this.canvas.width !== w || this.canvas.height !== h) {
      this.canvas.width = w;
      this.canvas.height = h;
    }

    gl.useProgram(this.program);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, w, h);

    // Blur kernel is in output-pixel units, so effects read the same relative to
    // the rendered size — matching the previous per-size processing behaviour.
    if (this.uTextureSize) gl.uniform2f(this.uTextureSize, w, h);
    for (const name of UNIFORM_NAMES) {
      gl.uniform1f(this.uniforms[name], adjustments[ADJUSTMENT_FOR_UNIFORM[name]] as number);
    }

    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    return this.canvas;
  }

  dispose(): void {
    const gl = this.gl;
    gl.deleteTexture(this.texture);
    gl.deleteProgram(this.program);
    const lose = gl.getExtension("WEBGL_lose_context");
    lose?.loseContext();
  }
}
