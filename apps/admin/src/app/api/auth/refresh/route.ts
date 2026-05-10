import { NextRequest, NextResponse } from "next/server";

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || "http://localhost:8080";
const BACKEND_REFRESH_PATH = "/admin/v1/auth/refresh";

export async function POST(request: NextRequest) {
  try {
    const refreshToken = request.cookies.get("admin_refresh_token")?.value;

    if (!refreshToken) {
      return NextResponse.json(
        { error: "No refresh token" },
        { status: 401 }
      );
    }

    const backendRes = await fetch(`${BACKEND_URL}${BACKEND_REFRESH_PATH}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${refreshToken}`,
      },
    });

    if (!backendRes.ok) {
      const response = NextResponse.json(
        { error: "Token refresh failed" },
        { status: 401 }
      );
      response.cookies.delete("admin_access_token");
      response.cookies.delete("admin_refresh_token");
      return response;
    }

    const data = await backendRes.json();
    const { access_token, refresh_token, ...claims } = data;

    const response = NextResponse.json(claims);

    response.cookies.set("admin_access_token", access_token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
      maxAge: 900,
      path: "/",
    });

    if (refresh_token) {
      response.cookies.set("admin_refresh_token", refresh_token, {
        httpOnly: true,
        secure: process.env.NODE_ENV === "production",
        sameSite: "strict",
        maxAge: 604800,
        path: "/api/auth/refresh",
      });
    }

    return response;
  } catch (error) {
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 }
    );
  }
}
