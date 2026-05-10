import { cookies } from 'next/headers';
import { NextResponse } from 'next/server';

const BACKEND_URL = process.env.GOGOMAIL_BACKEND_URL || 'http://localhost:8080';

export async function POST(request: Request) {
  try {
    const cookieStore = await cookies();
    const token = cookieStore.get('admin_access_token')?.value;

    if (!token) {
      return NextResponse.json(
        { error: 'Unauthorized' },
        { status: 401 }
      );
    }

    const body = await request.json();
    const { username, password } = body;

    if (!username || !password) {
      return NextResponse.json(
        { error: 'Username and password are required' },
        { status: 400 }
      );
    }

    // Call backend to complete initial setup
    const backendRes = await fetch(`${BACKEND_URL}/admin/v1/auth/setup`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ username, password }),
    });

    if (!backendRes.ok) {
      const errorData = await backendRes.json();
      return NextResponse.json(
        { error: errorData.error || 'Setup failed' },
        { status: backendRes.status }
      );
    }

    return NextResponse.json(
      { success: true },
      { status: 200 }
    );
  } catch (error) {
    console.error('Setup error:', error);
    return NextResponse.json(
      { error: 'Setup failed' },
      { status: 500 }
    );
  }
}
