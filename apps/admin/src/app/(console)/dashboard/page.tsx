"use client";

import { useQuery } from "@tanstack/react-query";
import Container from "@cloudscape-design/components/container";
import Header from "@cloudscape-design/components/header";
import Spinner from "@cloudscape-design/components/spinner";
import Box from "@cloudscape-design/components/box";

export default function DashboardPage() {
  const { data: stats, isLoading } = useQuery({
    queryKey: ["dashboard", "statistics"],
    queryFn: async () => {
      const res = await fetch("/api/admin/dashboard/statistics", {
        credentials: "include",
      });
      if (!res.ok) throw new Error("Failed to fetch statistics");
      return res.json();
    },
    staleTime: 5 * 60 * 1000,
  });

  return (
    <Box padding="l">
      <Container header={<Header variant="h1">Dashboard</Header>}>
        {isLoading ? (
          <Box textAlign="center" padding="l">
            <Spinner />
          </Box>
        ) : (
          <Box>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <div className="bg-slate-700 p-6 rounded-lg">
                <div className="text-sm text-gray-400">Total Users</div>
                <div className="text-3xl font-bold">
                  {stats?.total_users ?? 0}
                </div>
              </div>
              <div className="bg-slate-700 p-6 rounded-lg">
                <div className="text-sm text-gray-400">Active Sessions</div>
                <div className="text-3xl font-bold">
                  {stats?.active_sessions ?? 0}
                </div>
              </div>
              <div className="bg-slate-700 p-6 rounded-lg">
                <div className="text-sm text-gray-400">Mail Operations</div>
                <div className="text-3xl font-bold">
                  {stats?.mail_operations ?? 0}
                </div>
              </div>
              <div className="bg-slate-700 p-6 rounded-lg">
                <div className="text-sm text-gray-400">Audit Logs (24h)</div>
                <div className="text-3xl font-bold">
                  {stats?.audit_logs_24h ?? 0}
                </div>
              </div>
            </div>
          </Box>
        )}
      </Container>
    </Box>
  );
}
