import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { LinkFormDialog } from "@/components/LinkFormDialog";
import { request, type Link } from "@/lib/api";
import { LinkDetailPage } from "@/pages/app/LinkDetailPage";
import { LinksPage } from "@/pages/app/LinksPage";

/** Kembali ke halaman sebelumnya kalau ada (mis. dibuka dari Overview), selain itu ke fallback. */
function useCloseTo(fallback: string) {
  const navigate = useNavigate();
  return () => ((window.history.state?.idx ?? 0) > 0 ? navigate(-1) : navigate(fallback, { replace: true }));
}

/** /app/links/new — modal di atas halaman Links (Figma "New link — Desktop"). */
export function NewLinkRoute() {
  const navigate = useNavigate();
  const close = useCloseTo("/app/links");
  return (
    <>
      <LinksPage />
      <LinkFormDialog open onClose={close} onSaved={(l) => navigate(`/app/links/${l.id}`, { replace: true })} />
    </>
  );
}

/** /app/links/:id/edit — modal edit di atas Link detail. */
export function EditLinkRoute() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const link = useQuery({ queryKey: ["links", "detail", id], queryFn: () => request<Link>(`/links/${id}`) });
  const back = () => navigate(`/app/links/${id}`, { replace: true });
  return (
    <>
      <LinkDetailPage />
      {link.data && <LinkFormDialog key={link.data.updated_at} open link={link.data} onClose={back} onSaved={back} />}
    </>
  );
}
