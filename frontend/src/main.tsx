import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";
import { routes } from "./app/routes";
import { ThemeProvider } from "./lib/theme/ThemeProvider";
import "./styles/index.css";

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Missing #root element in index.html");

createRoot(rootElement).render(
  <StrictMode>
    <ThemeProvider>
      <RouterProvider router={createBrowserRouter(routes)} />
    </ThemeProvider>
  </StrictMode>,
);
