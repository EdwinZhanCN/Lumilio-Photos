import { Component, ErrorInfo, ReactNode } from "react";
import ErrorFallBack from "@/pages/ErrorFallBack";

interface ErrorBoundaryProps {
    children: ReactNode;
    fallback?: ReactNode | ((props: FallbackProps) => ReactNode);
}

interface ErrorBoundaryState {
    hasError: boolean;
    error: Error | null;
}

interface FallbackProps {
    error: Error | null;
    resetErrorBoundary: () => void;
}

export default class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
    constructor(props: ErrorBoundaryProps) {
        super(props);
        this.state = { hasError: false, error: null };
        this.resetErrorBoundary = this.resetErrorBoundary.bind(this);
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return { hasError: true, error };
    }

    componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
        console.error("ErrorBoundary caught an error", error, errorInfo);
    }

    resetErrorBoundary(): void {
        this.setState({ hasError: false, error: null });
    }

    render(): ReactNode {
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