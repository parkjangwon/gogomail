import { NextRequest, NextResponse } from "next/server";

export async function POST(_request: NextRequest) {
  const response = NextResponse.json({ message: "Logged out" });

  response.cookies.delete("admin_access_token");
  response.cookies.delete("admin_refresh_token");

  return response;
}
