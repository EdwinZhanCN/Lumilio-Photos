import React from "react";
import { useUpload } from "@/contexts/UploadContext";

function UploadNotifications() {
    const { error, success, hint } = useUpload();

    return (
        <div className="toast toast-top toast-right duration-500">
            {error && (
                <div className="alert alert-error">
                    {error}
                </div>
            )}
            {success && (
                <div className="alert alert-success">
                    {success}
                </div>
            )}
            {hint && (
                <div className="alert alert-info">
                    {hint}
                </div>
            )}
        </div>
    );
}

export default UploadNotifications;