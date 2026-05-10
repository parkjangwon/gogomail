import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || "http://localhost:8080";

async function silentRefresh(): Promise<string | null> {
  try {
    const res = await fetch(`${BACKEND_URL}/admin/v1/auth/refresh`, {
      method: "POST",
    });

    if (!res.ok) return null;

    const data = await res.json();
    return data.access_token ?? null;
  } catch {
    return null;
  }
}

async function proxy(
  request: NextRequest,
  segments: string[]
): Promise<NextResponse> {
  const cookieStore = await cookies();
  let token = cookieStore.get("admin_access_token")?.value;

  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const url = `${BACKEND_URL}/admin/v1/${segments.join("/")}${request.nextUrl.search}`;

  const body =
    ["GET", "HEAD"].includes(request.method) ? undefined : await request.text();

  let res = await fetch(url, {
    method: request.method,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body,
    cache: "no-store",
  });

  if (res.status === 401) {
    token = (await silentRefresh()) ?? undefined;
    if (!token) {
      return NextResponse.json({ error: "Session expired" }, { status: 401 });
    }

    res = await fetch(url, {
      method: request.method,
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body,
      cache: "no-store",
    });
  }

  const responseData = await res.json().catch(() => ({}));
  return NextResponse.json(responseData, { status: res.status });
}

export async function GET(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, params.path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, params.path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, params.path);
}

export async function PATCH(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, params.path);
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: { path: string[] } }
) {
  return proxy(request, params.path);
}
