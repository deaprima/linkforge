import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { cn } from "cn";
import { useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";
import { CloseIcon } from "@/components/brand/Icons";
import { Kliko } from "@/components/brand/Kliko";
import { PageHeader, PanelMessage } from "@/components/layout/AppLayout";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, sheetClass } from "@/components/ui/dialog";
import { Button, Field } from "@/components/ui/primitives";
import { ApiError, request, type ApiKey, type CreatedApiKey } from "@/lib/api";
import { formatDate, timeAgo } from "@/lib/format";

/** Kerangka modal LinkForge (Figma "Modal — New link" / "Key created"): header, body, footer berborder. */
function Modal({
  open,
  onClose,
  title,
  description,
  closable = true,
  mascot,
  children,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description: string;
  closable?: boolean;
  /** Kliko yang bertengger di tepi atas modal (Figma "Where Kliko appears": API key created). */
  mascot?: ReactNode;
  children: ReactNode;
  footer: ReactNode;
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && closable && onClose()}>
      <DialogContent
        showCloseButton={false}
        onInteractOutside={(e) => !closable && e.preventDefault()}
        onEscapeKeyDown={(e) => !closable && e.preventDefault()}
        className={sheetClass}
      >
        {mascot && <div className="pointer-events-none absolute right-4 bottom-full sm:right-6 translate-y-[6.7%]">{mascot}</div>}
        <div className="flex items-center justify-between border-b border-ink px-4 py-4 sm:px-6 sm:py-4.5">
          <DialogTitle className="text-xl font-semibold tracking-[-0.2px] text-ink">{title}</DialogTitle>
          {closable && (
            <DialogClose aria-label="Close" className="cursor-pointer p-0.5 focus-visible:outline-2 focus-visible:outline-signal">
              <CloseIcon />
            </DialogClose>
          )}
        </div>
        <DialogDescription className="sr-only">{description}</DialogDescription>
        {children}
        <div className="flex gap-2.5 border-t border-ink px-4 py-4 sm:justify-end sm:px-6 [&>button]:flex-1 sm:[&>button]:flex-none">{footer}</div>
      </DialogContent>
    </Dialog>
  );
}

function NewKeyDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (k: CreatedApiKey) => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const create = useMutation({
    mutationFn: (n: string) => request<CreatedApiKey>("/auth/api-keys", { method: "POST", body: { name: n } }),
    onSuccess: (k) => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      setName("");
      onCreated(k);
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Couldn't create the key."),
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return setError("Give the key a name so you can recognise it later.");
    if (name.trim().length > 100) return setError("At most 100 characters.");
    setError(null);
    create.mutate(name.trim());
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New key"
      description="Name a new API key."
      footer={
        <>
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="new-key-form" loading={create.isPending}>
            Create key
          </Button>
        </>
      }
    >
      <form id="new-key-form" onSubmit={onSubmit} noValidate className="flex flex-col gap-4.5 p-6">
        <Field
          label="Name"
          autoFocus
          placeholder="e.g. Raycast extension"
          value={name}
          onChange={(e) => setName(e.target.value)}
          helper="Where you'll use it, so you know which key to revoke later."
          error={error}
        />
      </form>
    </Modal>
  );
}

/** Figma "Modal — Key created" (18:565): key mentah ditampilkan SEKALI; tidak bisa ditutup tanpa konfirmasi. */
function KeyCreatedDialog({ created, onClose }: { created: CreatedApiKey | null; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    if (!created) return;
    try {
      await navigator.clipboard.writeText(created.key);
      setCopied(true);
      toast.success("Key copied");
    } catch {
      toast.error("Couldn't copy. Select the key and copy it manually.");
    }
  }
  return (
    <Modal
      open={created !== null}
      onClose={onClose}
      closable={false}
      mascot={<Kliko pose="key" className="w-[120px] sm:w-[150px]" />}
      title="Copy your new key"
      description="The full API key is shown only once."
      footer={
        <Button onClick={onClose} disabled={!copied} title={copied ? undefined : "Copy the key first"}>
          I’ve saved it
        </Button>
      }
    >
      <div className="flex flex-col gap-4 p-4 sm:p-6">
        <p className="text-[15px] leading-[1.5] text-graphite">This is the only time the full key is shown. LinkForge stores a hash, so it can’t show it again.</p>
        <div className="hidden flex-col gap-1 sm:flex">
          <p className="text-xs text-graphite">Name</p>
          <p className="text-[15px] font-semibold text-ink">{created?.name}</p>
        </div>
        <div className="flex flex-col gap-3 border border-signal bg-signal-soft p-3 sm:flex-row sm:items-center sm:py-3 sm:pr-3 sm:pl-4">
          <code className="min-w-0 flex-1 font-mono text-sm font-medium break-all text-ink select-all">{created?.key}</code>
          <Button size="sm" className="w-full sm:w-auto" onClick={copy}>
            {copied ? "Copied" : "Copy key"}
          </Button>
        </div>
        <div className="flex border border-ink bg-paper">
          <span aria-hidden className="w-1.5 shrink-0 self-stretch bg-stop" />
          <p className="px-3.5 py-3 text-[13px] leading-[1.45] text-ink">
            Store it in a password manager or your CI secrets. If you lose it, revoke it and create a new one.
          </p>
        </div>
      </div>
    </Modal>
  );
}

function RevokeDialog({ apiKey, onClose }: { apiKey: ApiKey | null; onClose: () => void }) {
  const qc = useQueryClient();
  const revoke = useMutation({
    mutationFn: (id: string) => request<void>(`/auth/api-keys/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      toast.success(`"${apiKey?.name}" revoked`);
      onClose();
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't revoke the key."),
  });
  return (
    <Modal
      open={apiKey !== null}
      onClose={onClose}
      title={`Revoke "${apiKey?.name}"?`}
      description="Revoking a key stops it immediately."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button className="border-stop bg-stop text-paper" loading={revoke.isPending} onClick={() => apiKey && revoke.mutate(apiKey.id)}>
            Revoke key
          </Button>
        </>
      }
    >
      <p className="p-6 text-[15px] leading-[1.5] text-graphite">Requests using this key fail immediately. Your other keys keep working. This can’t be undone.</p>
    </Modal>
  );
}

const th = "px-4 py-2.5 text-left text-xs font-semibold whitespace-nowrap text-graphite";
const td = "px-4 py-3.5";

export function ApiKeysPage() {
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreatedApiKey | null>(null);
  const [revoking, setRevoking] = useState<ApiKey | null>(null);
  const keys = useQuery({ queryKey: ["api-keys"], queryFn: () => request<ApiKey[]>("/auth/api-keys") });

  return (
    <>
      <PageHeader
        title="API keys"
        subtitle={
          <>
            Send a key in the <code className="font-mono text-[13px] text-ink">X-API-Key</code> header to create and manage links from scripts.
          </>
        }
        mobileSubtitle="below"
        actions={<Button onClick={() => setCreating(true)}>New key</Button>}
      />

      <section className="overflow-x-auto border border-ink bg-paper">
        {keys.isPending ? (
          <PanelMessage>Loading…</PanelMessage>
        ) : keys.isError ? (
          <PanelMessage tone="error">Couldn't load your keys. {keys.error.message}</PanelMessage>
        ) : keys.data.length === 0 ? (
          <PanelMessage>No keys yet. Create one to shorten links from your scripts, CLI or CI.</PanelMessage>
        ) : (
          <>
          {/* Mobile (Figma 21:351): nama, prefix, terakhir dipakai, dan Revoke di kanan */}
          <ul className="md:hidden">
            {keys.data.map((k) => (
              <li key={k.id} className="flex items-center justify-between gap-3 border-t border-ink px-4 py-3.5 first:border-t-0">
                <div className="flex min-w-0 flex-col gap-0.5">
                  <p className="truncate text-[15px] font-semibold text-ink">{k.name}</p>
                  <p className="font-mono text-xs text-ink">{k.key_prefix}••••••••</p>
                  <p className="text-xs text-graphite">{k.last_used_at ? `Used ${timeAgo(new Date(k.last_used_at))}` : "Never used"}</p>
                </div>
                <Button variant="outline" size="sm" className="shrink-0" onClick={() => setRevoking(k)}>
                  Revoke
                </Button>
              </li>
            ))}
          </ul>
          <table className="hidden w-full min-w-[720px] border-collapse md:table">
            <thead>
              <tr>
                <th className={th}>Name</th>
                <th className={cn(th, "w-[220px]")}>Key</th>
                <th className={cn(th, "w-40")}>Last used</th>
                <th className={cn(th, "w-40")}>Created</th>
                <th className={cn(th, "w-[120px]")}>
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {keys.data.map((k) => (
                <tr key={k.id} className="border-t border-ink">
                  <td className={cn(td, "text-sm font-semibold text-ink")}>{k.name}</td>
                  {/* Hanya prefix yang tersimpan; sisanya disamarkan seperti di Figma */}
                  <td className={cn(td, "font-mono text-xs whitespace-nowrap text-ink")}>{k.key_prefix}••••••••</td>
                  <td className={cn(td, "text-sm whitespace-nowrap", k.last_used_at ? "text-ink" : "text-graphite")}>
                    {k.last_used_at ? timeAgo(new Date(k.last_used_at)) : "Never"}
                  </td>
                  <td className={cn(td, "text-sm whitespace-nowrap text-ink")}>{formatDate(k.created_at)}</td>
                  <td className={cn(td, "py-2 text-right")}>
                    <Button variant="outline" size="sm" onClick={() => setRevoking(k)}>
                      Revoke
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </>
        )}
      </section>

      <p className="text-sm text-graphite">
        Keys never expire on their own. Revoking a key stops it immediately<span className="hidden sm:inline">; your other keys keep working</span>.
      </p>

      {creating && (
        <NewKeyDialog
          open
          onClose={() => setCreating(false)}
          onCreated={(k) => {
            setCreating(false);
            setCreated(k);
          }}
        />
      )}
      <KeyCreatedDialog created={created} onClose={() => setCreated(null)} />
      <RevokeDialog apiKey={revoking} onClose={() => setRevoking(null)} />
    </>
  );
}
