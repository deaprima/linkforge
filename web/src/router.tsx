import { createBrowserRouter, Navigate } from "react-router";
import { AppLayout } from "@/components/layout/AppLayout";
import { GuestOnly, RequireAuth } from "@/lib/auth";
import { ApiKeysPage } from "@/pages/app/ApiKeysPage";
import { LinkDetailPage } from "@/pages/app/LinkDetailPage";
import { EditLinkRoute, NewLinkRoute } from "@/pages/app/LinkModalRoutes";
import { LinksPage } from "@/pages/app/LinksPage";
import { OverviewPage } from "@/pages/app/OverviewPage";
import { ProfilePage } from "@/pages/app/ProfilePage";
import { RegisterPage, SignInPage } from "@/pages/auth/AuthPages";
import { LandingPage } from "@/pages/landing/LandingPage";
import { ExpiredPage, NotFoundPage, PasswordGatePage, UnavailablePage } from "@/pages/public/PublicPages";

/*
 * Routing web app (lihat API-CONTRACT.md "Routing web app"):
 *   /            landing page
 *   /app/*       halaman aplikasi (short link hidup di root, jadi app tidak boleh di root)
 * Path /app/login, /app/dashboard, /app/p/:alias, /app/not-found, /app/expired, /app/unavailable dipakai Gateway
 * sebagai target redirect — jangan diganti tanpa mengubah gateway/internal/handler/webpaths.go.
 */
export const router = createBrowserRouter([
  { path: "/", element: <LandingPage /> },
  { path: "/app", element: <Navigate to="/app/dashboard" replace /> },
  { path: "/app/login", element: <GuestOnly><SignInPage /></GuestOnly> },
  { path: "/app/register", element: <GuestOnly><RegisterPage /></GuestOnly> },
  {
    element: <RequireAuth><AppLayout /></RequireAuth>,
    children: [
      { path: "/app/dashboard", element: <OverviewPage /> },
      { path: "/app/links", element: <LinksPage /> },
      { path: "/app/links/new", element: <NewLinkRoute /> },
      { path: "/app/links/:id", element: <LinkDetailPage /> },
      { path: "/app/links/:id/edit", element: <EditLinkRoute /> },
      { path: "/app/api-keys", element: <ApiKeysPage /> },
      { path: "/app/profile", element: <ProfilePage /> },
    ],
  },
  { path: "/app/p/:alias", element: <PasswordGatePage /> },
  { path: "/app/not-found", element: <NotFoundPage /> },
  { path: "/app/expired", element: <ExpiredPage /> },
  { path: "/app/unavailable", element: <UnavailablePage /> },
  { path: "*", element: <NotFoundPage /> },
]);
