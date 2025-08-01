import React from "react";
import { FallbackProps } from "react-error-boundary";

type ErrorFallBackProps = {
  code: string | number;
  title: string;
  message?: string;
  error?: Error;
  resetErrorBoundary?: () => void;
};

/**
 * ErrorFallBack component
 * @param code - Error code to display e.g. 404, 500
 * @param title - Error title to display, short
 * @param message - Error message to display, long
 * @param error - Error object
 * @param resetErrorBoundary - Reset Button Function
 * @returns {React.ReactElement}
 * @constructor
 */
export default function ErrorFallBack({
  code,
  title,
  message,
  error,
  resetErrorBoundary,
}: ErrorFallBackProps & FallbackProps): React.ReactElement {
  return (
    <main className="grid min-h-full place-items-center bg-white px-6 py-24 sm:py-32 lg:px-8">
      <div className="text-center">
        <p className="text-base font-semibold text-indigo-600">{code}</p>
        <h1 className="mt-4 text-5xl font-semibold tracking-tight text-balance text-gray-900 sm:text-7xl">
          {title}
        </h1>
        <p className="mt-6 text-lg font-medium text-pretty text-gray-500 sm:text-xl/8">
          {message || error?.message}
        </p>
        <div className="mt-10 flex items-center justify-center gap-x-6">
          <a
            href="/"
            className="rounded-md bg-indigo-600 px-3.5 py-2.5 text-sm font-semibold text-white shadow-xs hover:bg-indigo-500 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
          >
            Go back home
          </a>
          <a
            onClick={resetErrorBoundary}
            className="rounded-md bg-transparent px-3.5 py-2.5 text-sm font-semibold text-red shadow-xs hover:bg-gray-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-300"
          >
            Try again
          </a>
          <a
            href="https://github.com/EdwinZhanCN/Lumilio-Photos"
            className="text-sm font-semibold text-gray-900"
          >
            Report an Issue <span aria-hidden="true">&rarr;</span>
          </a>
        </div>
      </div>
    </main>
  );
}
