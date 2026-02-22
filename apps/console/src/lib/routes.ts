export const ROUTES = {
  dashboard: "/",
  migrations: "/migrations",
  migrationDetail: (id: string) => `/migrations/${id}`,
  newMigration: "/migrations/new",
  preview: (migrationId: string, candidateId: string) => `/migrations/${migrationId}/preview/${candidateId}`,
  runDetail: (id: string) => `/runs/${id}`,
} as const;
