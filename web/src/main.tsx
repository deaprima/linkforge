import "@fontsource-variable/instrument-sans/index.css";
import "@fontsource/martian-mono/400.css";
import "@fontsource/martian-mono/500.css";
import "@fontsource/martian-mono/600.css";
import "@fontsource/martian-mono/700.css";
import "./index.css";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import { ApiError } from "@/lib/api";
import { ThemeProvider } from "@/lib/theme";
import { router } from "@/router";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Error 4xx (mis. 404, 401) tidak akan berubah dengan retry
      retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        <Toaster />
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
);
