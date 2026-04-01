// import 'package:flutter/material.dart';
// import 'package:nutrix/login.dart';
// import 'package:nutrix/sign_up.dart';

// void main() {
//   runApp(const NutriXApp());
// }

// class NutriXApp extends StatelessWidget {
//   const NutriXApp({super.key});

//   @override
//   Widget build(BuildContext context) {
//     return MaterialApp(
//       debugShowCheckedModeBanner: false,
//       title: 'NutriX',
//       theme: ThemeData(
//         useMaterial3: true,
//         // Font mặc định, bạn có thể thay bằng Google Fonts (ví dụ: Poppins)
//         fontFamily: 'Roboto',
//       ),
//       home: const WelcomeScreen(),
//     );
//   }
// }

// class WelcomeScreen extends StatelessWidget {
//   const WelcomeScreen({super.key});

//   // Màu chủ đạo lấy từ hình ảnh mẫu (Lime Green)
//   final Color primaryColor = const Color.fromARGB(255, 117, 218, 248);
//   final Color darkColor = const Color(0xFF1A1A1A);

//   @override
//   Widget build(BuildContext context) {
//     return Scaffold(
//       backgroundColor: darkColor,
//       body: Stack(
//         children: [
//           // 1. PHẦN NỀN HỌA TIẾT (Background Pattern)
//           // Vẽ các hình vuông mờ xoay nghiêng để tạo chiều sâu như hình mẫu
//           const Positioned(
//             top: -50,
//             left: -50,
//             child: BackgroundShape(size: 200, rotation: 0.2),
//           ),
//           const Positioned(
//             top: 100,
//             right: -80,
//             child: BackgroundShape(size: 250, rotation: -0.2),
//           ),
//           const Positioned(
//             bottom: 200,
//             left: -40,
//             child: BackgroundShape(size: 180, rotation: 0.1),
//           ),

//           const Positioned(
//             bottom: 0,
//             right: -40,
//             child: BackgroundShape(size: 300, rotation: -0.3),
//           ),
//           // 2. PHẦN NỘI DUNG CHÍNH
//           SafeArea(
//             child: Padding(
//               padding: const EdgeInsets.symmetric(horizontal: 24.0),
//               child: Column(
//                 mainAxisAlignment: MainAxisAlignment.center,
//                 children: [
//                   const Spacer(flex: 3), // Đẩy logo xuống một chút
//                   // --- LOGO & SLOGAN ---
//                   Row(
//                     mainAxisAlignment: MainAxisAlignment.center,
//                     children: [
//                       // Icon giả lập (hoặc thay bằng Image.asset)
//                       Icon(
//                         Icons.fitness_center,
//                         size: 40,
//                         color: const Color(0xFFE0C146),
//                       ),
//                       const SizedBox(width: 10),
//                       Text(
//                         'NutriX',
//                         style: TextStyle(
//                           fontSize: 48,
//                           fontWeight: FontWeight.w900, // Rất đậm
//                           color: const Color(0xFFE0C146),
//                           letterSpacing: -1.5,
//                         ),
//                       ),
//                     ],
//                   ),
//                   const SizedBox(height: 16),
//                   Text(
//                     'Revitalize Your Lifestyle\nwith NutriX app',
//                     textAlign: TextAlign.center,
//                     style: TextStyle(
//                       fontSize: 16,
//                       fontWeight: FontWeight.w500,
//                       color: const Color(0xFFE046D9).withValues(alpha: 0.8),
//                       height: 1.5,
//                     ),
//                   ),

//                   const Spacer(flex: 4), // Khoảng trống lớn ở giữa
//                   // --- CÁC NÚT BẤM (BUTTONS) ---

//                   // Nút 1: "I'm new here" (Nền đen, chữ trắng)
//                   SizedBox(
//                     width: double.infinity,
//                     height: 56,
//                     child: ElevatedButton(
//                       onPressed: () {
//                         Navigator.of(context).push(
//                           MaterialPageRoute(
//                             builder: (BuildContext context) {
//                               return const SignUpApp();
//                             },
//                           ), // TODO: Replace with SignUp page
//                         );
//                       },
//                       style: ElevatedButton.styleFrom(
//                         backgroundColor: darkColor,
//                         foregroundColor: Colors.white,
//                         elevation: 0,
//                         shape: RoundedRectangleBorder(
//                           borderRadius: BorderRadius.circular(30),
//                         ),
//                       ),
//                       child: const Text(
//                         "I'm new here",
//                         style: TextStyle(
//                           fontSize: 16,
//                           fontWeight: FontWeight.bold,
//                         ),
//                       ),
//                     ),
//                   ),

//                   const SizedBox(height: 16),

//                   // Nút 2: "I've been here before" (Nền trắng, chữ đen)
//                   SizedBox(
//                     width: double.infinity,
//                     height: 56,
//                     child: ElevatedButton(
//                       onPressed: () {
//                         Navigator.of(context).push(
//                           MaterialPageRoute(
//                             builder: (BuildContext context) {
//                               return const LoginApp();
//                             },
//                           ), // TODO: Replace with LoginApp page
//                         );
//                       },
//                       style: ElevatedButton.styleFrom(
//                         backgroundColor: Colors.white,
//                         foregroundColor: darkColor,
//                         elevation: 0,
//                         shape: RoundedRectangleBorder(
//                           borderRadius: BorderRadius.circular(30),
//                         ),
//                       ),
//                       child: const Text(
//                         "I've been here before",
//                         style: TextStyle(
//                           fontSize: 16,
//                           fontWeight: FontWeight.bold,
//                         ),
//                       ),
//                     ),
//                   ),

//                   const Spacer(flex: 1),

//                   // --- FOOTER (Terms & Policy) ---
//                   Padding(
//                     padding: const EdgeInsets.only(bottom: 20),
//                     child: Text(
//                       'By signing up to this app you agree with our\nTerms of Use and Privacy Policy',
//                       textAlign: TextAlign.center,
//                       style: TextStyle(
//                         fontSize: 11,
//                         color: const Color.fromARGB(255, 255, 255, 255).withValues(alpha: 0.6),
//                         height: 1.4,
//                       ),
//                     ),
//                   ),
//                 ],
//               ),
//             ),
//           ),
//         ],
//       ),
//     );
//   }
// }

// // Widget phụ để vẽ các hình nền mờ (Background Shapes)
// class BackgroundShape extends StatelessWidget {
//   final double size;
//   final double rotation;

//   const BackgroundShape({
//     super.key,
//     required this.size,
//     required this.rotation,
//   });

//   @override
//   Widget build(BuildContext context) {
//     return Transform.rotate(
//       angle: rotation,
//       child: Container(
//         width: size,
//         height: size,
//         decoration: BoxDecoration(
//           color: Colors.white12, // Màu trắng mờ
//           borderRadius: BorderRadius.circular(40),
//         ),
//       ),
//     );
//   }
// }
