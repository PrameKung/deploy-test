import type { NextRequest } from "next/server";

export const dynamic = "force-dynamic";

type RouteContext = { params: Promise<{ path: string[] }> };

async function proxy(request: NextRequest, context: RouteContext) {
  const backendURL = process.env.BACKEND_PUBLIC_URL ?? "http://localhost:8080";
  const { path } = await context.params;
  const target = new URL(
    `${path.map(encodeURIComponent).join("/")}${request.nextUrl.search}`,
    `${backendURL.replace(/\/$/, "")}/`,
  );
  const headers = new Headers(request.headers);
  headers.delete("host");
  headers.delete("content-length");

  const upstream = await fetch(target, {
    method: request.method,
    headers,
    body: request.method === "GET" || request.method === "HEAD"
      ? undefined
      : await request.arrayBuffer(),
    cache: "no-store",
    redirect: "manual",
  });

  const responseHeaders = new Headers();
  upstream.headers.forEach((value, name) => {
    if (!["connection", "content-encoding", "content-length", "set-cookie", "transfer-encoding"].includes(name.toLowerCase())) {
      responseHeaders.set(name, value);
    }
  });
  for (const cookie of upstream.headers.getSetCookie()) {
    responseHeaders.append("set-cookie", cookie);
  }

  const body = request.method === "HEAD" || upstream.status === 204 || upstream.status === 304
    ? null
    : upstream.body;
  return new Response(body, { status: upstream.status, headers: responseHeaders });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const OPTIONS = proxy;
