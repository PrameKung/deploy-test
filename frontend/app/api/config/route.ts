export const dynamic = "force-dynamic";

export async function GET() {
  return Response.json(
    { backendURL: process.env.BACKEND_PUBLIC_URL ?? "http://localhost:8080" },
    { headers: { "Cache-Control": "no-store" } },
  );
}
