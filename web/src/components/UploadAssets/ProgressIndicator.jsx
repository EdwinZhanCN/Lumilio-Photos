import React from 'react';

const ProgressIndicator = ({ processed, total, label }) => {
    return (
        <div className="mb-4">
            <div className="flex items-center gap-2">
                <progress
                    className="progress w-56"
                    value={processed}
                    max={total}
                ></progress>
                <span className="text-sm text-gray-500">
                  {processed}/{total} {label && ` - ${label}`}
                </span>
            </div>
        </div>
    );
};

export default ProgressIndicator;