import { redirect } from "next/navigation";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { Dashboard } from "@/components/dashboard/Dashboard";

export default async function HomePage() {
  const session = await getServerSession(authOptions);
  if (!session) redirect("/api/auth/signin");
  return <Dashboard />;
}
