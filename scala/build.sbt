name := "gamestats-scala"
version := "0.1"
scalaVersion := "2.13.12"

libraryDependencies ++= Seq(
  "com.lihaoyi" %% "upickle" % "3.1.0",
  "com.lihaoyi" %% "os-lib" % "0.9.1",
  "org.scalameta" %% "munit" % "0.7.29" % Test
)

testFrameworks += new TestFramework("munit.Framework")