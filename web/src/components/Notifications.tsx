import { useGlobal } from "@/contexts/GlobalContext";

function Notifications() {
    const { error, success, hint, info } = useGlobal();

    return (
        <div>
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
                {info && (
                    <div className="alert alert-info">
                        {info}
                    </div>
                )}
            </div>
            <div className="toast toast-bottom toast-left duration-500">
                {hint && (
                    <div className="alert alert-info">
                        {hint}
                    </div>
                )}
            </div>
        </div>

    );
}

export default Notifications;