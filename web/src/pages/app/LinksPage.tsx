import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { cn } from "cn";
import { useState } from "react";
import { Link as RouterLink, useNavigate, useSearchParams } from "react-router";
import { toast } from "sonner";
import { CopyIcon, LockIcon, MoreIcon } from "@/components/brand/Icons";
import { Kliko } from "@/components/brand/Kliko";
import { PageHeader, PanelMessage } from "@/components/layout/AppLayout";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Button, Chip } from "@/components/ui/primitives";
import { API_PREFIX, ApiError, requestWithMeta, request, type Link } from "@/lib/api";
import { displayUrl, formatDate, formatNumber, isExpired } from "@/lib/format";

const PAGE_SIZE = 20;

export async function copyShortUrl(url: string) {
  try {
    await navigator.clipboard.writeText(url);
    toast.success("Short link copied", { description: url });
  } catch {
    toast.error("Couldn't copy. Select the link and copy it manually.");
  }
}

export function useDeleteLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => request<void>(`/links/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["links"] });
      qc.invalidateQueries({ queryKey: ["analytics"] });
    },
  });
}

/** Dialog konfirmasi hapus — aksi destruktif tidak bisa dibatalkan. */
export function DeleteLinkDialog({ link, onClose, onDeleted }: { link: Link | null; onClose: () => void; onDeleted?: () => void }) {
  const del = useDeleteLink();
  return (
    <Dialog open={link !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="border-ink shadow-none">
        <DialogHeader>
          <DialogTitle>Delete /{link?.alias}?</DialogTitle>
          <DialogDescription>The short link stops working immediately and its alias becomes available to others. This can't be undone.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button
            size="sm"
            className="border-stop bg-stop text-paper"
            loading={del.isPending}
            onClick={() =>
              link &&
              del.mutate(link.id, {
                onSuccess: () => {
                  toast.success(`/${link.alias} deleted`);
                  onClose();
                  onDeleted?.();
                },
                onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't delete the link."),
              })
            }
          >
            Delete link
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RowActions({ link, onDelete }: { link: Link; onDelete: (l: Link) => void }) {
  const navigate = useNavigate();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Actions for /${link.alias}`}
        className="flex size-8 cursor-pointer items-center justify-center hover:bg-grid focus-visible:outline-2 focus-visible:outline-signal"
      >
        <MoreIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="border-ink shadow-none">
        <DropdownMenuItem onSelect={() => navigate(`/app/links/${link.id}`)}>View analytics</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate(`/app/links/${link.id}/edit`)}>Edit</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => copyShortUrl(link.short_url)}>Copy short link</DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={`${API_PREFIX}/links/${link.id}/qr?size=512`} download={`${link.alias}.png`}>
            Download QR code
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => onDelete(link)}>
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const th = "px-4 py-2.5 text-left text-xs font-semibold whitespace-nowrap text-graphite";
const td = "px-4 py-3.5";

export function LinksPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const page = Math.max(1, Number(params.get("page")) || 1);
  const [deleting, setDeleting] = useState<Link | null>(null);

  const links = useQuery({
    queryKey: ["links", page],
    queryFn: () => requestWithMeta<Link[]>(`/links?page=${page}&limit=${PAGE_SIZE}`),
    placeholderData: keepPreviousData,
  });

  const total = links.data?.meta?.total ?? 0;
  const rows = links.data?.data ?? [];
  const from = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const to = Math.min(page * PAGE_SIZE, total);
  const goTo = (p: number) => setParams(p === 1 ? {} : { page: String(p) });

  // Desktop: di dalam panel; mobile: di bawah daftar kartu (Figma 20:371)
  const pager = (
    <div className="flex items-center justify-between">
      <p className="text-[13px] text-graphite">
        {from}–{to} of {formatNumber(total)}
      </p>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => goTo(page - 1)}>
          Previous
        </Button>
        <Button variant="outline" size="sm" disabled={to >= total} onClick={() => goTo(page + 1)}>
          Next
        </Button>
      </div>
    </div>
  );

  return (
    <>
      <PageHeader
        title="Links"
        subtitle={links.data ? `${formatNumber(total)} ${total === 1 ? "link" : "links"}` : " "}
        actions={<Button onClick={() => navigate("/app/links/new")}>New link</Button>}
      />

      <section className="overflow-x-auto border border-ink bg-paper">
        {links.isPending ? (
          <PanelMessage>Loading…</PanelMessage>
        ) : links.isError ? (
          <PanelMessage tone="error">Couldn't load your links. {links.error.message}</PanelMessage>
        ) : rows.length === 0 ? (
          <div className="flex flex-col items-center gap-4 px-5 py-10 text-center">
            <Kliko pose="empty" className="w-[160px]" />
            <p className="text-sm text-graphite">You haven't shortened anything yet.</p>
            <Button onClick={() => navigate("/app/links/new")}>Create your first link</Button>
          </div>
        ) : (
          <>
          {/* Mobile (Figma 20:371): satu kartu per link */}
          <ul className="md:hidden">
            {rows.map((l) => (
              <li key={l.id} className="flex flex-col gap-1.5 border-t border-ink px-4 py-3.5 first:border-t-0">
                <div className="flex items-center justify-between gap-3">
                  <span className="flex min-w-0 items-center gap-2">
                    <RouterLink to={`/app/links/${l.id}`} className="truncate font-mono text-sm font-medium text-ink hover:underline">
                      /{l.alias}
                    </RouterLink>
                    <button
                      type="button"
                      aria-label={`Copy ${l.short_url}`}
                      className="shrink-0 cursor-pointer p-0.5 focus-visible:outline-2 focus-visible:outline-signal"
                      onClick={() => copyShortUrl(l.short_url)}
                    >
                      <CopyIcon />
                    </button>
                  </span>
                  <span className="shrink-0 text-[13px] font-semibold text-ink">{formatNumber(l.click_count)} clicks</span>
                </div>
                <p className="truncate text-[13px] text-graphite">{displayUrl(l.original_url)}</p>
                {(isExpired(l.expired_at) || l.expired_at || l.has_password) && (
                  <div className="flex flex-wrap gap-2 pt-1">
                    {isExpired(l.expired_at) ? <Chip tone="stop">Expired</Chip> : l.expired_at && <Chip>Expires {formatDate(l.expired_at)}</Chip>}
                    {l.has_password && <Chip>Password</Chip>}
                  </div>
                )}
              </li>
            ))}
          </ul>
          <table className="hidden w-full min-w-[820px] border-collapse md:table">
            <thead>
              <tr>
                <th className={cn(th, "w-[220px]")}>Alias</th>
                <th className={th}>Destination</th>
                <th className={cn(th, "w-[90px] text-right")}>Clicks</th>
                <th className={cn(th, "w-[170px]")}>Expires</th>
                <th className={cn(th, "w-[110px]")}>Password</th>
                <th className={cn(th, "w-16")}>
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((l) => (
                <tr key={l.id} className="border-t border-ink hover:bg-grid/60">
                  <td className={td}>
                    <span className="flex items-center gap-2">
                      <RouterLink to={`/app/links/${l.id}`} className="font-mono text-[13px] whitespace-nowrap text-ink hover:underline">
                        /{l.alias}
                      </RouterLink>
                      <button
                        type="button"
                        aria-label={`Copy ${l.short_url}`}
                        className="cursor-pointer p-0.5 focus-visible:outline-2 focus-visible:outline-signal"
                        onClick={() => copyShortUrl(l.short_url)}
                      >
                        <CopyIcon />
                      </button>
                    </span>
                  </td>
                  <td className={cn(td, "max-w-0")}>
                    <span className="block truncate text-sm text-graphite" title={l.original_url}>
                      {displayUrl(l.original_url)}
                    </span>
                  </td>
                  <td className={cn(td, "text-right text-sm font-semibold text-ink")}>{formatNumber(l.click_count)}</td>
                  <td className={cn(td, "text-sm whitespace-nowrap text-ink")}>
                    {isExpired(l.expired_at) ? <Chip tone="stop">Expired</Chip> : l.expired_at ? formatDate(l.expired_at) : "Never"}
                  </td>
                  <td className={td}>
                    {l.has_password ? (
                      <span className="flex items-center gap-2 text-sm text-ink">
                        <LockIcon />
                        On
                      </span>
                    ) : (
                      <span className="text-sm text-graphite">Off</span>
                    )}
                  </td>
                  <td className={cn(td, "py-2 text-right")}>
                    <RowActions link={l} onDelete={setDeleting} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </>
        )}
        {total > 0 && <div className="hidden border-t border-ink px-4 py-3 md:block">{pager}</div>}
      </section>
      {total > 0 && <div className="md:hidden">{pager}</div>}

      <DeleteLinkDialog link={deleting} onClose={() => setDeleting(null)} />
    </>
  );
}
