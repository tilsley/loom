export const ROUTES = {
  dashboard: "/",
  migrations: "/migrations",
  migrationDetail: (id: string) => `/migrations/${id}`,
  newMigration: "/migrations/new",
  runDetail: (id: string) => `/runs/${id}`,
} as const;
