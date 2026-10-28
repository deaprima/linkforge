import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router";
import { ApiError, request, type User } from "@/lib/api";

const ME_KEY = ["me"] as const;

/** User yang sedang login, atau null kalau belum login (401 setelah refresh gagal). */
export function useMe() {
  return useQuery({
    queryKey: ME_KEY,
    queryFn: async () => {
      try {
        return await request<User>("/auth/me");
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null;
        throw err;
      }
    },
    staleTime: 5 * 60_000,
    retry: false,
  });
}

type AuthResponse = { user: User };

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { email: string; password: string }) =>
      request<AuthResponse>("/auth/login", { method: "POST", body: input, noRefresh: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ME_KEY }),
  });
}

export function useRegister() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; email: string; password: string }) =>
      request<AuthResponse>("/auth/register", { method: "POST", body: input, noRefresh: true }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ME_KEY }),
  });
}

/** Selalu sukses untuk email terdaftar maupun tidak (Gateway sengaja membalas sama). */
export function useForgotPassword() {
  return useMutation({
    mutationFn: (input: { email: string }) => request<{ message: string }>("/auth/forgot-password", { method: "POST", body: input, noRefresh: true }),
  });
}

export function useResetPassword() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { token: string; password: string }) => request<{ message: string }>("/auth/reset-password", { method: "POST", body: input, noRefresh: true }),
    // Semua sesi dicabut di server; buang juga sisa data akun di cache
    onSuccess: () => {
      qc.setQueryData(ME_KEY, null);
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== "me" });
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    // Refresh token dibaca Gateway dari cookie; body kosong
    mutationFn: () => request<void>("/auth/logout", { method: "POST", noRefresh: true }),
    onSettled: () => {
      qc.setQueryData(ME_KEY, null);
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== "me" });
    },
  });
}

/** Path tujuan setelah login: ?next= hanya boleh path internal /app (cegah open redirect). */
export function safeNext(next: string | null): string {
  return next && next.startsWith("/app/") && !next.startsWith("//") ? next : "/app/dashboard";
}

/** Guard halaman aplikasi: tamu diarahkan ke /app/login?next=<halaman ini>. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { data: user, isPending } = useMe();
  const location = useLocation();
  if (isPending) return null;
  if (!user) {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/app/login?next=${next}`} replace />;
  }
  return <>{children}</>;
}

/** Halaman tamu (login/register): user yang sudah login langsung ke dashboard. */
export function GuestOnly({ children }: { children: ReactNode }) {
  const { data: user, isPending } = useMe();
  const location = useLocation();
  if (isPending) return null;
  if (user) return <Navigate to={safeNext(new URLSearchParams(location.search).get("next"))} replace />;
  return <>{children}</>;
}
