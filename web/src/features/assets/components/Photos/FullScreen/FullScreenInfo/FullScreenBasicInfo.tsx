import { SquarePen, X } from "lucide-react";

export default function FullScreenBasicInfo() {
  return (
    <div className="absolute top-20 right-0 z-10 font-mono">
      <div className="card bg-base-100 w-max shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">Basic Info</h1>
            <div className="badge badge-soft badge-success">OK</div>
            {/* TODO: Edit Basic Info Functionality, Now disable*/}
            <button className="btn btn-circle btn-xs" disabled>
              <SquarePen className="w-4 h-4" />
            </button>
            <button className="btn btn-circle btn-xs">
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="px-2">
            <div className="flex">
              <p>2025-8-26 19:02</p>
              <div className="text-xs text-info">HEIF</div>
            </div>
            <div className="flex">
              <p>IMG_1675</p>
            </div>
          </div>
          <div className="rounded bg-base-300">
            <div className="px-2 py-0.5">
              <p>Apple iPhone 13 mini</p>
              <p>广角摄像头-26mm f1.6</p>
              <div className="flex justify-between">
                <p>12MP</p>
                <p>4032 * 3024</p>
                <p>2.4M</p>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 p-2">
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                ISO 100
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                1/60s
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                0ev
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                26mm
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                f/1.6
              </div>
            </div>
          </div>
          {/* TODO: Face Avatar */}
          {/* TODO: Map */}
          <div className="card-actions justify-end font-sans">
            <button className="btn btn-sm btn-soft btn-primary">
              View EXIF
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
