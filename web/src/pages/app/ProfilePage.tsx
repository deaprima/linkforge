import type { ReactNode } from "react";
import { useNavigate } from "react-router";
import { PageHeader } from "@/components/layout/AppLayout";
import { Button } from "@/components/ui/primitives";
import { useLogout, useMe } from "@/lib/auth";
import { formatDate } from "@/lib/format";

/** Baris detail akun (Figma "Account details"): label graphite 180px + nilai. */
function Row({ label, children, first }: { label: string; children: ReactNode; first?: boolean }) {
  return (
    <div className={first ? "flex flex-col gap-1 px-5 py-4 sm:flex-row sm:items-center sm:gap-4" : "flex flex-col gap-1 border-t border-ink px-5 py-4 sm:flex-row sm:items-center sm:gap-4"}>
      <p className="text-sm text-graphite sm:w-[180px] sm:shrink-0">{label}</p>
      <div className="min-w-0 truncate text-[15px] text-ink">{children}</div>
    </div>
  );
}

// Catatan: baris "Sign-in methods" di Figma belum ditampilkan — /auth/me belum mengembalikan
// metode login (password / Google). Butuh field baru di proto User + Auth Service.
export function ProfilePage() {
  const { data: user } = useMe();
  const logout = useLogout();
  const navigate = useNavigate();

  return (
    <>
      <PageHeader title="Profile" subtitle="Your account details." />
      <section aria-label="Account details" className="flex w-full max-w-[720px] flex-col border border-ink bg-paper">
        <Row label="Name" first>
          {user?.name}
        </Row>
        <Row label="Email">{user?.email}</Row>
        <Row label="Member since">{user?.created_at ? formatDate(user.created_at) : "—"}</Row>
      </section>
      <div className="flex max-w-[560px] flex-col items-start gap-3 pt-2">
        <p className="text-sm text-graphite">Signing out ends this session. Other devices stay signed in until their sessions expire.</p>
        <Button variant="outline" loading={logout.isPending} onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/app/login", { replace: true }) })}>
          Sign out
        </Button>
      </div>
    </>
  );
}
