import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { toast } from "sonner";
import { CloseIcon } from "@/components/brand/Icons";
import { FormError } from "@/components/layout/AuthLayout";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button, Field } from "@/components/ui/primitives";
import { ApiError, request, type Link } from "@/lib/api";

/*
 * Modal "New link" (Figma 17:672) — juga dipakai untuk edit.
 * State "alias taken" (Figma 17:871): field alias merah + "This alias is already taken. Try another one."
 */

const ALIAS_RE = /^[a-zA-Z0-9_-]{3,64}$/;

/**
 * Prefix alias = host short link, mis. "linkforge.deaprima.my.id/". Di produksi short link & web app
 * satu domain; saat dev short link hidup di Gateway (VITE_SHORT_URL_BASE, lihat .env.development).
 */
function shortHost(link?: Link): string {
  if (link) return link.short_url.replace(/^https?:\/\//, "").replace(/[^/]+$/, "");
  const base = import.meta.env.VITE_SHORT_URL_BASE as string | undefined;
  return `${(base ?? window.location.origin).replace(/^https?:\/\//, "").replace(/\/$/, "")}/`;
}

/** <input type="date"> → akhir hari itu (waktu lokal) dalam ISO, supaya link berlaku sepanjang tanggal tsb. */
function endOfDayISO(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(y, m - 1, d, 23, 59, 59).toISOString();
}

function toDateInput(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

type Errors = { url?: string; alias?: string; expiry?: string; password?: string; form?: string };

export function LinkFormDialog({ open, link, onClose, onSaved }: { open: boolean; link?: Link; onClose: () => void; onSaved?: (l: Link) => void }) {
  const editing = Boolean(link);
  const qc = useQueryClient();
  const [url, setUrl] = useState(link?.original_url ?? "");
  const [alias, setAlias] = useState(link?.alias ?? "");
  const [expiry, setExpiry] = useState(toDateInput(link?.expired_at ?? null));
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<Errors>({});

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      editing ? request<Link>(`/links/${link!.id}`, { method: "PUT", body }) : request<Link>("/links", { method: "POST", body }),
    onSuccess: (saved) => {
      qc.invalidateQueries({ queryKey: ["links"] });
      qc.invalidateQueries({ queryKey: ["analytics"] });
      toast.success(editing ? "Link updated" : "Short link created", { description: saved.short_url });
      onSaved?.(saved);
    },
    onError: (err) => {
      if (!(err instanceof ApiError)) return setErrors({ form: "Something went wrong. Please try again." });
      switch (err.code) {
        case "ALIAS_ALREADY_TAKEN":
          return setErrors({ alias: "This alias is already taken. Try another one." });
        case "MALICIOUS_URL":
          return setErrors({ url: "This domain is blocked for safety reasons." });
        case "VALIDATION_ERROR":
          return setErrors(err.message.includes("alias") ? { alias: err.message } : { url: err.message });
        default:
          return setErrors({ form: err.code === "SERVICE_UNAVAILABLE" ? "LinkForge is temporarily unavailable. Please try again." : err.message });
      }
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const next: Errors = {};
    try {
      const u = new URL(url.trim());
      if (u.protocol !== "http:" && u.protocol !== "https:") throw new Error();
    } catch {
      next.url = "Must start with http:// or https://";
    }
    if (alias && !ALIAS_RE.test(alias)) next.alias = "3–64 characters: letters, numbers, - and _ only.";
    if (expiry && new Date(endOfDayISO(expiry)) <= new Date()) next.expiry = "Pick a date in the future.";
    if (password && (password.length < 4 || password.length > 72)) next.password = "Between 4 and 72 characters.";
    setErrors(next);
    if (Object.keys(next).length) return;

    // PUT = replace: expired_at tidak dikirim berarti expiry dihapus (lihat API-CONTRACT.md)
    save.mutate({
      original_url: url.trim(),
      ...(alias && { alias }),
      ...(expiry && { expired_at: endOfDayISO(expiry) }),
      ...(!editing && password && { password }),
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent showCloseButton={false} className="w-[calc(100%-2rem)] max-w-[560px] gap-0 border-ink p-0 shadow-none sm:max-w-[560px]">
        <div className="flex items-center justify-between border-b border-ink px-6 py-4.5">
          <DialogTitle className="text-xl font-semibold tracking-[-0.2px] text-ink">{editing ? "Edit link" : "New link"}</DialogTitle>
          <DialogClose aria-label="Close" className="cursor-pointer p-0.5 focus-visible:outline-2 focus-visible:outline-signal">
            <CloseIcon />
          </DialogClose>
        </div>
        <DialogDescription className="sr-only">{editing ? "Change where this short link points." : "Shorten a long URL."}</DialogDescription>

        <form onSubmit={onSubmit} noValidate>
          <div className="flex flex-col gap-4.5 p-6">
            {errors.form && <FormError>{errors.form}</FormError>}
            <Field label="Destination URL" type="url" inputMode="url" placeholder="https://" autoFocus value={url} onChange={(e) => setUrl(e.target.value)} error={errors.url} />
            <Field
              label="Custom alias (optional)"
              prefix={shortHost(link)}
              value={alias}
              onChange={(e) => {
                setAlias(e.target.value);
                if (errors.alias) setErrors((x) => ({ ...x, alias: undefined }));
              }}
              helper={editing ? "Changing it breaks the old short link." : "Leave empty and LinkForge will generate one."}
              error={errors.alias}
              autoCapitalize="off"
              spellCheck={false}
            />
            <div className="flex flex-col gap-4 sm:flex-row">
              <Field
                className="flex-1"
                label="Expiry date (optional)"
                type="date"
                value={expiry}
                onChange={(e) => setExpiry(e.target.value)}
                helper="After this date visitors see that the link expired."
                error={errors.expiry}
              />
              {!editing && (
                <Field
                  className="flex-1"
                  label="Password (optional)"
                  type="password"
                  autoComplete="new-password"
                  placeholder="No password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  helper="Visitors enter it before they are redirected."
                  error={errors.password}
                />
              )}
            </div>
          </div>
          <div className="flex justify-end gap-2.5 border-t border-ink px-6 py-4">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" loading={save.isPending}>
              {editing ? "Save changes" : "Create link"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
