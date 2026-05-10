import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || "http://localhost:8080";
const BACKEND_AUTH_PATH = "/admin/v1/auth/login";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();

    const backendRes = await fetch(`${BACKEND_URL}${BACKEND_AUTH_PATH}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!backendRes.ok) {
      const error = await backendRes.json().catch(() => ({}));
      return NextResponse.json(
        error || { error: "Authentication failed" },
        { status: backendRes.status }
      );
    }

    const data = await backendRes.json();
    const { access_token, refresh_token, ...claims } = data;

    const response = NextResponse.json(claims, { status: 200 });

    response.cookies.set("admin_access_token", access_token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
      maxAge: 900, // 15 minutes
      path: "/",
    });

    response.cookies.set("admin_refresh_token", refresh_token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
      maxAge: 604800, // 7 days
      path: "/api/auth/refresh",
    });

    return response;
  } catch (error) {
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 }
    );
  }
}
