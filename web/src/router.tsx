import type { ComponentType } from "react";
import { createBrowserRouter, Navigate, type RouteObject } from "react-router";
import { GuestOnly, RequireAuth } from "@/lib/auth";

/*
 * Routing web app (lihat API-CONTRACT.md "Routing web app"):
 *   /            landing page
 *   /app/*       halaman aplikasi (short link hidup di root, jadi app tidak boleh di root)
 * Path /app/login, /app/dashboard, /app/p/:alias, /app/not-found, /app/expired, /app/unavailable dipakai Gateway
 * sebagai target redirect — jangan diganti tanpa mengubah gateway/internal/handler/webpaths.go.
 *
 * Tiap halaman dimuat malas (code splitting): pengunjung landing atau password gate tidak mengunduh kode dashboard.
 */

/** Rute yang modulnya baru diunduh saat dikunjungi. `wrap` membungkus halaman (mis. guard auth). */
function lazyPage<M>(load: () => Promise<M>, pick: (m: M) => ComponentType, wrap?: (page: ComponentType) => ComponentType): Pick<RouteObject, "lazy"> {
  return {
    lazy: async () => {
      const page = pick(await load());
      return { Component: wrap ? wrap(page) : page };
    },
  };
}

const guest = (Page: ComponentType): ComponentType =>
  function Guest() {
    return (
      <GuestOnly>
        <Page />
      </GuestOnly>
    );
  };

const authed = (Layout: ComponentType): ComponentType =>
  function Authed() {
    return (
      <RequireAuth>
        <Layout />
      </RequireAuth>
    );
  };

const publicPages = () => import("@/pages/public/PublicPages");
const authPages = () => import("@/pages/auth/AuthPages");
const resetPages = () => import("@/pages/auth/PasswordResetPages");

export const router = createBrowserRouter([
  { path: "/", ...lazyPage(() => import("@/pages/landing/LandingPage"), (m) => m.LandingPage) },
  { path: "/app", element: <Navigate to="/app/dashboard" replace /> },
  { path: "/app/login", ...lazyPage(authPages, (m) => m.SignInPage, guest) },
  { path: "/app/register", ...lazyPage(authPages, (m) => m.RegisterPage, guest) },
  // Dokumentasi publik: dibaca developer sebelum punya akun
  { path: "/app/docs", ...lazyPage(() => import("@/pages/docs/DocsPage"), (m) => m.DocsPage) },
  { path: "/app/forgot-password", ...lazyPage(resetPages, (m) => m.ForgotPasswordPage, guest) },
  // Tanpa GuestOnly: link di email bisa dibuka di browser yang masih login; sesinya dicabut oleh reset itu sendiri
  { path: "/app/reset-password", ...lazyPage(resetPages, (m) => m.ResetPasswordPage) },
  {
    ...lazyPage(() => import("@/components/layout/AppLayout"), (m) => m.AppLayout, authed),
    children: [
      { path: "/app/dashboard", ...lazyPage(() => import("@/pages/app/OverviewPage"), (m) => m.OverviewPage) },
      { path: "/app/links", ...lazyPage(() => import("@/pages/app/LinksPage"), (m) => m.LinksPage) },
      { path: "/app/links/new", ...lazyPage(() => import("@/pages/app/LinkModalRoutes"), (m) => m.NewLinkRoute) },
      { path: "/app/links/:id", ...lazyPage(() => import("@/pages/app/LinkDetailPage"), (m) => m.LinkDetailPage) },
      { path: "/app/links/:id/edit", ...lazyPage(() => import("@/pages/app/LinkModalRoutes"), (m) => m.EditLinkRoute) },
      { path: "/app/api-keys", ...lazyPage(() => import("@/pages/app/ApiKeysPage"), (m) => m.ApiKeysPage) },
      { path: "/app/profile", ...lazyPage(() => import("@/pages/app/ProfilePage"), (m) => m.ProfilePage) },
    ],
  },
  { path: "/app/p/:alias", ...lazyPage(publicPages, (m) => m.PasswordGatePage) },
  { path: "/app/not-found", ...lazyPage(publicPages, (m) => m.NotFoundPage) },
  { path: "/app/expired", ...lazyPage(publicPages, (m) => m.ExpiredPage) },
  { path: "/app/unavailable", ...lazyPage(publicPages, (m) => m.UnavailablePage) },
  { path: "*", ...lazyPage(publicPages, (m) => m.NotFoundPage) },
]);
