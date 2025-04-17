import React from "react";
import ErrorFallBack from "@/pages/ErrorFallBack.jsx";

export default class ErrorBoundary extends React.Component {
    constructor(props) {
        super(props);
        this.state = { hasError: false, error: null };
        this.resetErrorBoundary = this.resetErrorBoundary.bind(this);
    }

    static getDerivedStateFromError(error) {
        return { hasError: true, error };
    }

    componentDidCatch(error, errorInfo) {
        console.error("ErrorBoundary caught an error", error, errorInfo);
    }

    resetErrorBoundary() {
        this.setState({ hasError: false, error: null });
    }

    render() {
        if (this.state.hasError) {
            if (typeof this.props.fallback === 'function') {
                return this.props.fallback({
                    error: this.state.error,
                    resetErrorBoundary: this.resetErrorBoundary
                });
            }
            return this.props.fallback ||
                <ErrorFallBack
                    code={"500"}
                    title={"Oops...Something Went Wrong."}
                    message={"An unexpected error occurred."}
                    reset={this.resetErrorBoundary}
                />;
        }
        return this.props.children;
    }
}