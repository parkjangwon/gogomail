import { NextResponse } from 'next/server';

const IS_PROD = process.env.NODE_ENV === 'production';

export async function POST() {
  const response = NextResponse.json({ ok: true });
  response.cookies.set('webmail_token', '', {
    httpOnly: true,
    secure: IS_PROD,
    sameSite: 'strict',
    path: '/',
    maxAge: 0,
  });
  return response;
}
