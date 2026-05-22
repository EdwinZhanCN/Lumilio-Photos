import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import RegistrationForm from "../components/RegistrationForm.tsx";

const BootstrapRegisterPage: React.FC = () => {
  const { t } = useI18n();

  return (
    <RegistrationForm
      credentialTitle={t("auth.bootstrap.register.title", {
        defaultValue: "Create the first Admin account",
      })}
      credentialSubtitle={t("auth.bootstrap.register.subtitle", {
        defaultValue:
          "Username first, then passkey or authenticator setup to finish Admin onboarding.",
      })}
      credentialSubmitLabel={t("auth.bootstrap.register.submit", {
        defaultValue: "Continue as Admin",
      })}
      credentialPrompt={{
        title: t("auth.bootstrap.register.promptTitle", {
          defaultValue: "You're registering the initial Admin user",
        }),
        body: t("auth.bootstrap.register.promptBody", {
          defaultValue:
            "Lumilio will create the first Admin account and require passkey or authenticator enrollment before the setup is complete.",
        }),
      }}
      showLoginLink={false}
    />
  );
};

export default BootstrapRegisterPage;
