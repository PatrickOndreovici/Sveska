import { render } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { routes } from "../app/routes";
import { AuthProvider } from "../features/auth/AuthProvider";
import { ThemeProvider } from "../lib/theme/ThemeProvider";

/** Renders the whole app (providers, layout, routes) at the given path. */
export function renderApp(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  render(
    <ThemeProvider>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </ThemeProvider>,
  );
  return router;
}
