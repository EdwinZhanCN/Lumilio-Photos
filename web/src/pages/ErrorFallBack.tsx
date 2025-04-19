import React from "react";


type ErrorFallBackProps = {
    code: string | number;
    title: string;
    message: string;
    reset?: () => void;
};

/**
 * ErrorFallBack component
 * @param code - Error code to display e.g. 404, 500
 * @param title - Error title to display, short
 * @param message - Error message to display, long
 * @param reset - Reset Button Function
 * @returns {React.ReactElement}
 * @constructor
 */
export default function ErrorFallBack({code, title, message, reset}:ErrorFallBackProps): React.ReactElement {
    return (
        <main className="grid min-h-full place-items-center bg-white px-6 py-24 sm:py-32 lg:px-8">
            <div className="text-center">
                <p className="text-base font-semibold text-indigo-600">{code}</p>
                <h1 className="mt-4 text-5xl font-semibold tracking-tight text-balance text-gray-900 sm:text-7xl">{title}</h1>
                <p className="mt-6 text-lg font-medium text-pretty text-gray-500 sm:text-xl/8">{message}</p>
                <div className="mt-10 flex items-center justify-center gap-x-6">
                    <a href="/"
                       className="rounded-md bg-indigo-600 px-3.5 py-2.5 text-sm font-semibold text-white shadow-xs hover:bg-indigo-500 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">Go
                        back home</a>
                    <button
                        onClick={reset}
                        className="btn btn-outline btn-error mt-2"
                    >
                        Try Again
                    </button>
                    <a href="#" className="text-sm font-semibold text-gray-900">Report an Issue <span
                        aria-hidden="true">&rarr;</span></a>
                </div>
            </div>
        </main>
    )
}