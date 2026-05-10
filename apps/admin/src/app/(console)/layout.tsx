"use client";

import { useEffect } from "react";
import AppLayout from "@cloudscape-design/components/app-layout";
import TopNavigation from "@cloudscape-design/components/top-navigation";
import SideNavigation from "@cloudscape-design/components/side-navigation";
import { applyMode, Mode } from "@cloudscape-design/global-styles";
import { useRouter } from "next/navigation";

export default function ConsoleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();

  useEffect(() => {
    applyMode(Mode.Dark);
  }, []);

  const handleLogout = async () => {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" });
    router.push("/login");
  };

  return (
    <>
      <TopNavigation
        identity={{
          href: "/dashboard",
          title: "GoGoMail Admin",
        }}
        utilities={[
          {
            type: "button",
            text: "Logout",
            onClick: () => handleLogout(),
          },
        ]}
      />
      <AppLayout
        navigation={
          <SideNavigation
            activeHref={typeof window !== "undefined" ? window.location.pathname : "/"}
            items={[
              { type: "link", text: "Dashboard", href: "/dashboard" },
              { type: "divider" },
              {
                type: "section",
                text: "Identity",
                items: [
                  { type: "link", text: "Users", href: "/users" },
                  { type: "link", text: "Organizations", href: "/organizations" },
                  { type: "link", text: "Roles", href: "/roles" },
                ],
              },
              {
                type: "section",
                text: "Domain",
                items: [
                  { type: "link", text: "Domains", href: "/domains" },
                  {
                    type: "link",
                    text: "Identity Providers",
                    href: "/identity-providers",
                  },
                ],
              },
              {
                type: "section",
                text: "Monitoring",
                items: [
                  { type: "link", text: "Mail Logs", href: "/mail-logs" },
                  { type: "link", text: "Audit Logs", href: "/audit-logs" },
                  { type: "link", text: "Statistics", href: "/statistics" },
                ],
              },
              {
                type: "section",
                text: "Compliance",
                items: [
                  {
                    type: "link",
                    text: "Audit Policy",
                    href: "/audit-policy",
                  },
                  { type: "link", text: "Reports", href: "/reports" },
                ],
              },
            ]}
          />
        }
        breadcrumbs={null}
        contentType="default"
        toolsHide
        content={children}
      />
    </>
  );
}
