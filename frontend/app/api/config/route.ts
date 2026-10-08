export const dynamic = "force-dynamic";

export async function GET() {
  return Response.json(
    { backendURL: "/api/backend" },
    { headers: { "Cache-Control": "no-store" } },
  );
}
