export const ROUTES = {
  dashboard: "/",
  migrations: "/migrations",
  migrationDetail: (id: string) => `/migrations/${id}`,
  preview: (migrationId: string, candidateId: string) => `/migrations/${migrationId}/preview/${candidateId}`,
  candidateSteps: (migrationId: string, candidateId: string) => `/migrations/${migrationId}/candidates/${candidateId}/steps`,
} as const;
