import { NextResponse } from "next/server";

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || "http://localhost:8080";

export async function POST() {
  try {
    // Call backend logout endpoint
    await fetch(`${BACKEND_URL}/admin/v1/auth/logout`, {
      method: "POST",
    }).catch(() => {
      // Logout can proceed even if backend call fails
    });

    // Clear auth cookies on client side
    const response = NextResponse.json({ success: true }, { status: 200 });

    response.cookies.delete("admin_access_token");
    response.cookies.delete("admin_refresh_token");

    return response;
  } catch (error) {
    return NextResponse.json(
      { error: "Logout failed" },
      { status: 500 }
    );
  }
}
