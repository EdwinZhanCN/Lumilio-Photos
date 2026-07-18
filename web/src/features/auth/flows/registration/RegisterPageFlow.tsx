import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import RegistrationForm from "./RegistrationForm.tsx";

const RegisterPage: React.FC = () => {
  const { t } = useI18n();

  return (
    <RegistrationForm
      credentialTitle={t("auth.register.title", {
        defaultValue: "Create an account",
      })}
      credentialSubtitle={t("auth.register.subtitle", {
        defaultValue: "Create your username and password first, then secure the account.",
      })}
      credentialSubmitLabel={t("auth.register.submit", {
        defaultValue: "Continue",
      })}
    />
  );
};

export default RegisterPage;
