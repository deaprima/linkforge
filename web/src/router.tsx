import { createBrowserRouter, Navigate } from "react-router";
import { GuestOnly, RequireAuth } from "@/lib/auth";
import { RegisterPage, SignInPage } from "@/pages/auth/AuthPages";
import { PendingPage } from "@/pages/PendingPage";

/*
 * Routing web app (lihat API-CONTRACT.md "Routing web app"):
 *   /            landing page
 *   /app/*       halaman aplikasi (short link hidup di root, jadi app tidak boleh di root)
 * Path /app/login, /app/dashboard, /app/p/:alias, /app/not-found, /app/expired dipakai Gateway
 * sebagai target redirect — jangan diganti tanpa mengubah gateway/internal/handler/webpaths.go.
 */
export const router = createBrowserRouter([
  { path: "/", element: <PendingPage title="Landing page" /> },
  { path: "/app", element: <Navigate to="/app/dashboard" replace /> },
  { path: "/app/login", element: <GuestOnly><SignInPage /></GuestOnly> },
  { path: "/app/register", element: <GuestOnly><RegisterPage /></GuestOnly> },
  { path: "/app/dashboard", element: <RequireAuth><PendingPage title="Overview" /></RequireAuth> },
  { path: "/app/p/:alias", element: <PendingPage title="Password gate" /> },
  { path: "/app/not-found", element: <PendingPage title="404" /> },
  { path: "/app/expired", element: <PendingPage title="410" /> },
  { path: "*", element: <PendingPage title="404" /> },
]);
