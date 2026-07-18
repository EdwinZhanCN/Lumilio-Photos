import { useEffect, useRef } from "react";

export function usePasswordConfirmation(
  password: string,
  confirmation: string,
  mismatchMessage: string,
) {
  const confirmationRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    const input = confirmationRef.current;
    if (!input) return;

    input.setCustomValidity(
      confirmation && confirmation !== password ? mismatchMessage : "",
    );
  }, [confirmation, mismatchMessage, password]);

  return confirmationRef;
}
